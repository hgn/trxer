package main

import "flag"
import "fmt"
import "os"
import "net"
import "time"
import "strconv"
import "sync"

var UPDATE_INTERVAL = 5
// 32k
var BYTE_BUFFER_SIZE = 8096 * 8

func udp_client(threads int, addr string) {
	//
}

func udp_server(threads int) {
	//
}

func tcp_client_worker(addr string, wg sync.WaitGroup) {

	buf := make([]byte, BYTE_BUFFER_SIZE, BYTE_BUFFER_SIZE)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic("dial")
	}
	for {
		_, err  := conn.Write(buf)
		if err != nil {
			panic("write")
		}
	}
	wg.Done()
}

func tcp_client(threads int, addr string) {
	port := 6666
	var wg sync.WaitGroup
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		listen := addr + ":" + strconv.Itoa(port);
		go tcp_client_worker(listen, wg)
		port += 1
	}
	wg.Wait()
}

type measurement struct {
	bytes uint64
	time  float64
}

func tcp_server(threads int) {

	c := make(chan measurement)
	port := 6666
	for i := 0; i < threads; i++ {
		go tcp_server_worker(c, port)
		port += 1
	}


	var accumulated uint64  = 0;
	for {
		for i := 0; i < threads; i++ {
			val := <-c
			accumulated += val.bytes
		}
		mbyte_sec := accumulated / (1000000 * uint64(UPDATE_INTERVAL))
		println("MByte/sec: ", mbyte_sec)
		accumulated = 0
	}
}

func tcp_server_worker(c chan<- measurement, port int) {

	listen := "localhost:" + strconv.Itoa(port);
	println("listening on ", listen)
	addr, error := net.ResolveTCPAddr("tcp", listen);
	if error != nil {
		fmt.Printf("Cannot parse \"%s\": %s\n", listen, error);
		os.Exit(1);
	}
	listener, error := net.ListenTCP("tcp", addr);
	if error != nil {
		fmt.Printf("Cannot listen: %s\n", error);
		os.Exit(1);
	}

	conn, error := listener.AcceptTCP();
	if error != nil {
		fmt.Printf("Cannot accept: %s\n", error);
		os.Exit(1);
	}

	fmt.Printf("Connection from %s\n", conn.RemoteAddr());
	message := make([]byte, BYTE_BUFFER_SIZE, BYTE_BUFFER_SIZE);

	var bytes uint64  = 0;
	start := time.Now()
	for {
		n1, error := conn.Read(message);
		if error != nil {
			fmt.Printf("Cannot read: %s\n", error);
			os.Exit(1);
		}

		bytes += uint64(n1)

		elapsed := time.Since(start)
		if elapsed.Seconds() > float64(UPDATE_INTERVAL) {
			c <- measurement{bytes: bytes, time: elapsed.Seconds()}
			start = time.Now()
			bytes = 0
		}
	}

}

func main() {

    protoPtr := flag.String("protocol", "udp", "udp or tcp")
    modePtr := flag.String("mode", "server", "server (\"localhost\") or IP address ")
    threadPtr := flag.Int("threads", 1, "an int for numer of coroutines")

    flag.Parse()
    fmt.Println("protocol: ", *protoPtr)
    fmt.Println("mode:     ", *modePtr)
    fmt.Println("threads:  ", *threadPtr)

    if (*protoPtr == "udp") {
	    if (*modePtr == "server") {
		    udp_server(*threadPtr)
	    } else {
		    udp_client(*threadPtr, *modePtr)
	    }
    } else if (*protoPtr == "tcp") {
	    if (*modePtr == "server") {
		    tcp_server(*threadPtr)
	    } else {
		    tcp_client(*threadPtr, *modePtr)
	    }
    } else {
	    panic("udp or tcp")
    }

}
