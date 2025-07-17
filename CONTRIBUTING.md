# Contributing

## Generating Proto types

```bash
cd proto
buf generate
```

## Contents of a block

```bash
firecore tools print merged-blocks /dir/location/to/merged-blocks 487625 -o jsonl --proto-paths /location/to/proto/folder --bytes-encoding base64 | jq .
```
