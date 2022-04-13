docker container kill test-connector
docker container rm test-connector
docker build -t test-connector .
docker run --rm  \
       --net=host \
       -v $(pwd)/../btcconnector:/btcconnector \
-it test-connector