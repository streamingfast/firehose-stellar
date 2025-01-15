firecore \
    start \
    substreams-tier2 \
    --config-file= \
    --log-format=stackdriver \
    --common-merged-blocks-store-url=data/merged \
    --common-first-streamable-block=0 \
    --common-one-block-store-url=data/oneblock \
    --substreams-tier1-grpc-listen-addr=9000 \
    --substreams-tier2-grpc-listen-addr=:9001 \
    --substreams-state-store-url=data/substreams-tier2/states \
    --substreams-state-bundle-size=100