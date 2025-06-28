#! /bin/sh

docker compose -f scripts/sut/mongodb.yaml -f scripts/sut/kafka.yaml down --remove-orphans --volumes

rm -f miner/app/data/resume_token.bin