# MathGen

## Concept:

Gemma 3n LLM assisted Math Word Problem Generator

Input Name and things you like and generate a personable problem set.

Todo: Add Templates for border around PDF
Todo: expand to Algebra, Graphs, time telling

Spin up server and publish 

## how to build and run
```
go mod tidy
./build.sh
cd ./build/
./mathgen
```

Must have ollama installed

TODO: Docker-Compose the process with seperate GRPC, web app, and ollama

```
$ ./mathgen --help 
Usage of ./mathgen:
  -grpc-port string
        gRPC server port (default ":50051")
  -model string
        model name to pass to Ollama (default "gemma3n:e4b")
  -ollama_url string
        base URL of Ollama API (default "http://localhost:11434")
  -out_dir string
        directory to write JSON + PDF results (default "./output")
  -web_port string
        port for Gin web UI (default ":8081")
```


