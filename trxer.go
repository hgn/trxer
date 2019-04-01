package main

import "flag"
import "fmt"
import "os"
import "net"
import "time"
import "strconv"
import "sync"


var UPDATE_INTERVAL = 5
var PORT = 6666
var DEF_BUFFER_SIZE = 8096


type measurement struct {
	bytes uint64
	time  float64
}


func udp_client_worker(addr string, wg *sync.WaitGroup, bufSize int) {
	defer wg.Done()
	buf := make([]byte, bufSize, bufSize)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		panic("dial")
	}
	defer conn.Close()

	for {
		_, err := conn.Write(buf)
		if err != nil {
			panic("write")
		}
	}
}

func udp_client(threads int, addr string, bufSize int) {
	port := PORT
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		listen := addr + ":" + strconv.Itoa(port)
		wg.Add(1)
		go udp_client_worker(listen, &wg, bufSize)
		port += 1
	}
	wg.Wait()
}

func udp_server_worker(c chan<- measurement, port int, bufSize int) {

	listen := "[::]:" + strconv.Itoa(port)
	addr, error := net.ResolveUDPAddr("udp", listen)
	if error != nil {
		fmt.Printf("Cannot listen: %s\n", error)
		os.Exit(1)
	}

	println("Listening on", listen)
	pc, error := net.ListenUDP("udp", addr)
	if error != nil {
		fmt.Printf("Cannot listen: %s\n", error)
		os.Exit(1)
	}
	defer pc.Close()

	message := make([]byte, bufSize, bufSize)

	var bytes uint64 = 0
	start := time.Now()
	for {
		read, _, error := pc.ReadFromUDP(message)
		if error != nil {
			fmt.Printf("Cannot read: %s\n", error)
			os.Exit(1)
		}

		bytes += uint64(read)

		elapsed := time.Since(start)
		if elapsed.Seconds() > float64(UPDATE_INTERVAL) {
			c <- measurement{bytes: bytes, time: elapsed.Seconds()}
			start = time.Now()
			bytes = 0
		}
	}

}

func udp_server(threads int, bufSize int) {
	c := make(chan measurement)
	port := 6666
	for i := 0; i < threads; i++ {
		go udp_server_worker(c, port, bufSize)
		port += 1
	}

	var accumulated uint64 = 0
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

func tcp_client_worker(addr string, wg *sync.WaitGroup, bufSize int) {
	defer wg.Done()
	buf := make([]byte, bufSize, bufSize)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic("dial")
	}
	defer conn.Close()

	for {
		_, err := conn.Write(buf)
		if err != nil {
			panic("write")
		}
	}
}

func tcp_client(threads int, addr string, bufSize int) {
	port := 6666
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		listen := addr + ":" + strconv.Itoa(port)
		wg.Add(1)
		go tcp_client_worker(listen, &wg, bufSize)
		port += 1
	}
	wg.Wait()
}

func tcp_server(threads int, bufSize int) {
	c := make(chan measurement)
	port := 6666
	for i := 0; i < threads; i++ {
		go tcp_server_worker(c, port, bufSize)
		port += 1
	}

	var accumulated uint64 = 0
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

func tcp_server_worker(c chan<- measurement, port int, bufSize int) {
	listen := "[::]:" + strconv.Itoa(port)
	println("Listening on", listen)
	addr, error := net.ResolveTCPAddr("tcp", listen)
	if error != nil {
		fmt.Printf("Cannot parse \"%s\": %s\n", listen, error)
		os.Exit(1)
	}
	listener, error := net.ListenTCP("tcp", addr)
	if error != nil {
		fmt.Printf("Cannot listen: %s\n", error)
		os.Exit(1)
	}
	defer listener.Close()

	conn, error := listener.AcceptTCP()
	if error != nil {
		fmt.Printf("Cannot accept: %s\n", error)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Connection from %s\n", conn.RemoteAddr())
	message := make([]byte, bufSize, bufSize)

	var bytes uint64 = 0
	start := time.Now()
	for {
		n1, error := conn.Read(message)
		if error != nil {
			fmt.Printf("Cannot read: %s\n", error)
			os.Exit(1)
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
	lengthPtr := flag.Int("length", DEF_BUFFER_SIZE, "application read/write buffer size in byte")

	flag.Parse()
	fmt.Println("trxer(c) - 2017")
	fmt.Println("Protocol:", *protoPtr)
	fmt.Println("Mode:", *modePtr)
	fmt.Println("Threads:", *threadPtr)
	fmt.Println("Buffer Length: ", *lengthPtr)

	if *protoPtr == "udp" {
		if *modePtr == "server" {
			udp_server(*threadPtr, *lengthPtr)
		} else {
			udp_client(*threadPtr, *modePtr, *lengthPtr)
		}
	} else if *protoPtr == "tcp" {
		if *modePtr == "server" {
			tcp_server(*threadPtr, *lengthPtr)
		} else {
			tcp_client(*threadPtr, *modePtr, *lengthPtr)
		}
	} else {
		panic("quic, udp or tcp")
	}

}
