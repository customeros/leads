#!/bin/bash
set -eo pipefail

PROTO_DIR="./customeros/packages/server/proto"

echo "Updating dependencies..."
go mod tidy

echo "Generating GraphQL code..."
go run github.com/99designs/gqlgen generate --config ./api/graphql/gqlgen.yml

echo "Generating protobuf code..."
find $PROTO_DIR -name "*.proto" -type f -exec \
protoc \
--proto_path=$PROTO_DIR \
--go_out=./internal/proto/pb \
--go_opt=paths=source_relative \
{} \;

echo "Building application..."
go build -v .

echo "Build completed successfully."

