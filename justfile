build:
    go build -o .bin/app .

db:
    pgcli postgresql://postgres:password@localhost:5555/leads

debug:
    dlv debug . -- server

gen-api:
    go run github.com/99designs/gqlgen generate --config ./api/graphql/gqlgen.yml

gen-proto:
    find ./proto -name "*.proto" -type f -exec \
    protoc \
    --proto_path=./proto \
    --go_out=./proto/pb \
    --go_opt=module=github.com/customeros/leads/proto/pb \
    --go-grpc_out=./proto/pb \
    --go-grpc_opt=module=github.com/customeros/leads/proto/pb \
    {} \;

run:
    go run main.go

tidy:
    go mod tidy

warehouse:
    pgcli postgresql://postgres:password@localhost:5556/warehouse


