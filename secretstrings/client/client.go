package main

import (
	//	"net/rpc"
	"bufio"
	"flag"
	"fmt"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/distributed2/secretstrings/stubs"
)

func main() {
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	fmt.Println("Server: ", *server)
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()

	request := stubs.Request{Message: "Hello"}
	response := new(stubs.Response)

	file, _ := os.Open("../wordlist")
	Scanner := bufio.NewScanner(file)
	Scanner.Split(bufio.ScanWords)

	for Scanner.Scan() {
		// fmt.Println(Scanner.Text())
		request.Message = Scanner.Text()
		client.Call(stubs.PremiumReverseHandler, request, response)
		fmt.Println("Response: " + response.Message)
	}

}
