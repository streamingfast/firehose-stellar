firecore start reader-node merger \
  --config-file= \
  --log-format=stackdriver \
  --log-to-file=false \
  --data-dir=data \
  --common-auto-max-procs  \
  --common-auto-mem-limit-percent=90  \
  --common-one-block-store-url=data/oneblock  \
  --common-first-streamable-block=741400  \
  --reader-node-data-dir=data/oneblock  \
  --reader-node-working-dir=data/work  \
  --reader-node-readiness-max-latency=600s  \
  --reader-node-debug-firehose-logs=false  \
  --reader-node-blocks-chan-capacity=1000  \
  --reader-node-grpc-listen-addr=:9001  \
  --reader-node-manager-api-addr=:8080  \
  --reader-node-path=firestellar  \
  --reader-node-arguments='fetch rpc 741400 \
  --state-dir data  \
  --endpoints https://soroban-testnet.stellar.org/'
