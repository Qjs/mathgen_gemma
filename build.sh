export GOPATH=$HOME/go  
export PATH=$PATH:$GOPATH/bin 

protoc --go_out=. --go-grpc_out=. ./server/proto/problem_gen.proto

go build -o build/mathgen