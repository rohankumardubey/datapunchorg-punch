#!/bin/bash

# echo commands to the terminal output
set -ex

namePrefix=my
instanceType=t3.xlarge
instanceCount=3

databaseUser=user1
databasePassword=password1
sparkApiGatewayUser=user1
sparkApiGatewayPassword=password1

./punch install RdsDatabase --set namePrefix=$namePrefix \
  --patch spec.masterUserName=$databaseUser --patch spec.masterUserPassword=$databasePassword \
  -o RdsDatabase.output.json

databaseEndpoint=$(jq -r '.output[] | select(.step=="createDatabase").output.endpoint' RdsDatabase.output.json)

./punch install Eks --set namePrefix=$namePrefix \
  --patch Spec.NodeGroups[0].InstanceTypes[0]=$instanceType \
  --patch Spec.NodeGroups[0].MinSize=$instanceCount \
  --patch Spec.NodeGroups[0].MaxSize=$instanceCount \
  --patch Spec.NodeGroups[0].DesiredSize=$instanceCount \
  -o Eks.output.json

./punch install HiveMetastore --set namePrefix=$namePrefix \
  --patch spec.database.externalDb=true \
  --patch spec.database.connectionString=jdbc:postgresql://${databaseEndpoint}:5432/postgres \
  --patch spec.database.user=$databaseUser --patch spec.database.password=$databasePassword \
  -o HiveMetastore.output.json

metastoreUri=$(jq -r '.output[] | select(.step=="installHiveMetastoreServer").output.metastoreLoadBalancerUrls[0]' HiveMetastore.output.json)
metastoreWarehouseDir=$(jq -r '.output[] | select(.step=="installHiveMetastoreServer").output.metastoreWarehouseDir' HiveMetastore.output.json)

./punch install SparkOnEks --set namePrefix=$namePrefix \
  --patch spec.spark.gateway.user=$sparkApiGatewayUser \
  --patch spec.spark.gateway.password=$sparkApiGatewayPassword \
  --patch spec.spark.gateway.hiveMetastoreUris=$metastoreUri \
  --patch spec.spark.gateway.sparkSqlWarehouseDir=$metastoreWarehouseDir \
  --print-notes \
  -o SparkOnEks.output.json

apiGatewayLoadBalancerUrl=$(jq -r '.output[] | select(.step=="installNginxIngressController").output.loadBalancerPreferredUrl' SparkOnEks.output.json)


echo Running test Spark application

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit -o SparkcliSubmit.output.json \
  --class org.apache.spark.examples.SparkPi \
  --spark-version 3.2 \
  --driver-memory 512m --executor-memory 512m \
  local:///opt/spark/examples/jars/spark-examples_2.12-3.2.1.jar

submissionId=$(jq -r '.submissionId' SparkcliSubmit.output.json)

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 status $submissionId

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 list

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 delete $submissionId


metastoreWarehouseDirS3Url=$(echo $metastoreWarehouseDir | sed -e "s/^s3a/s3/")

aws s3 ls $metastoreWarehouseDirS3Url/
aws s3 rm --recursive $metastoreWarehouseDirS3Url/punch_test_db_01.db
aws s3 ls $metastoreWarehouseDirS3Url/

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit \
  --spark-version 3.2 \
  --driver-memory 512m --executor-memory 512m \
  examples/pyspark-hive-example.py

# Install Kafka Bridge which is a REST service to produce Kafka messages

./punch install KafkaBridge --set namePrefix=$namePrefix -o KafkaBridge.output.json

bootstrapServerString=$(jq -r '.output[] | select(.step=="createKafkaCluster").output.bootstrapServerString' KafkaBridge.output.json)
kafkaBridgeTopicProduceUrl=$(jq -r '.output[] | select(.step=="deployStrimziKafkaBridge").output.kafkaBridgeTopicProduceUrl' KafkaBridge.output.json)

curl -k $kafkaBridgeTopicProduceUrl

curl -k -X POST $kafkaBridgeTopicProduceUrl/topic_01 -H 'Content-Type: application/vnd.kafka.json.v2+json' -d '{"records":[{"key":"key1","value":"value1"},{"key":"key2","value":"value2"}]}'

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit --class org.datapunch.sparkapp.KafkaIngestion \
  --spark-version 3.2 \
  --class org.datapunch.sparkapp.SparkSql s3a://datapunch-public-01/sparkapp/sparkapp-1.0.2.jar \
  --sql "show databases"

echo Submitting Spark steaming application to ingest Kafka data

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit --id kafka-ingestion --overwrite \
  --class org.datapunch.sparkapp.KafkaIngestion \
  --spark-version 3.2 \
  --driver-memory 1g --executor-memory 1g \
  --conf spark.jars=s3a://datapunch-public-01/jars/aws-msk-iam-auth-1.1.0-all.jar \
  --conf spark.kubernetes.submission.waitAppCompletion=false \
  s3a://datapunch-public-01/sparkapp/sparkapp-1.0.5-shaded.jar \
  --bootstrapServers $bootstrapServerString \
  --database my_msk_01_kafka_ingestion --topic topic_01 --triggerSeconds 20 --printTableData true \
  --kafkaOption kafka.security.protocol=SASL_SSL --kafkaOption kafka.sasl.mechanism=AWS_MSK_IAM \
  --kafkaOption kafka.sasl.jaas.config="software.amazon.msk.auth.iam.IAMLoginModule required;" \
  --kafkaOption kafka.sasl.client.callback.handler.class=software.amazon.msk.auth.iam.IAMClientCallbackHandler

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 list

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 status kafka-ingestion


kafkaJsonMessage='{"records":[{"key":"key1","value":"value1"},{"key":"key2","value":"value2"}]}'
echo Command example to send more data to Kafka: curl -k -X POST $kafkaBridgeTopicProduceUrl/topic_01 -H \'Content-Type: application/vnd.kafka.json.v2+json\' -d \'$kafkaJsonMessage\'


echo Installing Kyuubi

./punch install KyuubiOnEks --set namePrefix=$namePrefix \
  --patch spec.spark.gateway.user=$sparkApiGatewayUser \
  --patch spec.spark.gateway.password=$sparkApiGatewayPassword \
  --patch spec.spark.gateway.hiveMetastoreUris=$metastoreUri \
  --patch spec.spark.gateway.sparkSqlWarehouseDir=$metastoreWarehouseDir \
  -o KyuubiOnEks.output.json

echo KyuubiOnEks deployment output

cat KyuubiOnEks.output.json | jq .

./punch install Superset --set namePrefix=$namePrefix \
  --print-notes \
  -o Superset.output.json
