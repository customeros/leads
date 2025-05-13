proto_dir := "../customeros/packages/server/proto"

build:
    go build -o .bin/app .

db:
    pgcli postgresql://postgres:password@localhost:5555/leads

debug:
    dlv debug . -- server

gen-api:
    go run github.com/99designs/gqlgen generate --config ./api/graphql/gqlgen.yml

gen-proto:
    find {{proto_dir}} -name "*.proto" -type f -exec \
    protoc \
    --proto_path={{proto_dir}} \
    --go_out=./internal/proto/pb \
    --go_opt=paths=source_relative \
    {} \;

run:
    go run main.go

tidy:
    go mod tidy

warehouse:
    pgcli postgresql://postgres:password@localhost:5556/warehouse


