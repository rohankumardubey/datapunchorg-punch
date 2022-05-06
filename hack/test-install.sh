#!/bin/bash

export databaseUser=user1
export databasePassword=password1
export sparkApiGatewayUser=user1
export sparkApiGatewayPassword=password1

# echo commands to the terminal output
set -ex

./punch install RdsDatabase --patch spec.masterUserName=$databaseUser --patch spec.masterUserPassword=$databasePassword \
  -o RdsDatabase.output.json

export databaseEndpoint=$(jq -r '.output[] | select(.step=="createDatabase").output.endpoint' RdsDatabase.output.json)

./punch install HiveMetastore --patch spec.database.externalDb=true \
  --patch spec.database.connectionString=jdbc:postgresql://${databaseEndpoint}:5432/postgres \
  --patch spec.database.user=$databaseUser --patch spec.database.password=$databasePassword \
  -o HiveMetastore.output.json

export metastoreUri=$(jq -r '.output[] | select(.step=="installHiveMetastoreServer").output.metastoreLoadBalancerUrls[0]' HiveMetastore.output.json)
export metastoreWarehouseDir=$(jq -r '.output[] | select(.step=="installHiveMetastoreServer").output.metastoreWarehouseDir' HiveMetastore.output.json)

./punch install SparkOnEks --patch spec.apiGateway.userName=$sparkApiGatewayUser \
  --patch spec.apiGateway.userPassword=$sparkApiGatewayPassword \
  --patch spec.apiGateway.hiveMetastoreUris=$metastoreUri \
  --patch spec.apiGateway.sparkSqlWarehouseDir=$metastoreWarehouseDir \
  --print-usage-example \
  -o SparkOnEks.output.json

export apiGatewayLoadBalancerUrl=$(jq -r '.output[] | select(.step=="deployNginxIngressController").output.loadBalancerPreferredUrl' SparkOnEks.output.json)

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit --class org.apache.spark.examples.SparkPi \
  --spark-version 3.2 \
  --driver-memory 512m --executor-memory 512m \
  local:///opt/spark/examples/jars/spark-examples_2.12-3.2.1.jar

export metastoreWarehouseDirS3Url=$(echo $metastoreWarehouseDir | sed -e "s/^s3a/s3/")

aws s3 ls $metastoreWarehouseDirS3Url/
aws s3 rm --recursive $metastoreWarehouseDirS3Url/punch_test_db_01.db
aws s3 ls $metastoreWarehouseDirS3Url/

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit \
  --spark-version 3.2 \
  --driver-memory 512m --executor-memory 512m \
  examples/pyspark-hive-example.py



# Install Kafka with Bridge

./punch install KafkaWithBridge -o KafkaWithBridge.output.json

export bootstrapServerString=$(jq -r '.output[] | select(.step=="createKafkaCluster").output.bootstrapServerString' KafkaWithBridge.output.json)
export kafkaBridgeTopicProduceUrl=$(jq -r '.output[] | select(.step=="deployStrimziKafkaBridge").output.kafkaBridgeTopicProduceUrl' KafkaWithBridge.output.json)

curl -k $kafkaBridgeTopicProduceUrl

curl -k -X POST $kafkaBridgeTopicProduceUrl/topic_01 -H 'Content-Type: application/vnd.kafka.json.v2+json' -d '{"records":[{"key":"key1","value":"value1"},{"key":"key2","value":"value2"}]}'

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit --class org.datapunch.sparkapp.KafkaIngestion \
  --spark-version 3.2 \
  --driver-memory 1g --executor-memory 1g \
  --conf spark.jars=s3a://datapunch-public-01/jars/aws-msk-iam-auth-1.1.0-all.jar \
  s3a://datapunch-public-01/sparkapp/sparkapp-1.0.5-shaded.jar \
  --bootstrapServers b-1.my-msk-01.i5lvjr.c2.kafka.us-west-1.amazonaws.com:9098,b-2.my-msk-01.i5lvjr.c2.kafka.us-west-1.amazonaws.com:9098 \
  --database my_msk_01 --topic topic_01 --triggerSeconds 20 --printTableData true \
  --kafkaOption kafka.security.protocol=SASL_SSL --kafkaOption kafka.sasl.mechanism=AWS_MSK_IAM \
  --kafkaOption kafka.sasl.jaas.config="software.amazon.msk.auth.iam.IAMLoginModule required;" \
  --kafkaOption kafka.sasl.client.callback.handler.class=software.amazon.msk.auth.iam.IAMClientCallbackHandler

./sparkcli --user $sparkApiGatewayUser --password $sparkApiGatewayPassword --insecure \
  --url ${apiGatewayLoadBalancerUrl}/sparkapi/v1 submit --class org.datapunch.sparkapp.KafkaIngestion \
  --spark-version 3.2 \
  --class org.datapunch.sparkapp.SparkSql s3a://datapunch-public-01/sparkapp/sparkapp-1.0.2.jar \
  --sql "show databases"
