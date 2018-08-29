package main

import "flag"
import "fmt"
import "os"
import "net"
import "time"
import "strconv"
import "sync"
//import "reflect"

// quic specific packages
import "crypto/tls"
import quic "github.com/lucas-clemente/quic-go"
import "crypto/rsa"
import "crypto/rand"
import "crypto/x509"
import "math/big"
import "encoding/pem"
import "io"


var UPDATE_INTERVAL = 5
var PORT = 6666

var BYTE_BUFFER_SIZE = 8096 * 8

type measurement struct {
	bytes uint64
	time  float64
}

func quic_client_worker(addr string, wg *sync.WaitGroup) {
	fmt.Println("Quic stream connecting to: ", addr)

	buf := make([]byte, 1400, 1400)

	/* create tls conf, true => TLS accepts any certificate presented
	by the server and any host name in that certificate
	 */
	
	tlsConf := tls.Config {InsecureSkipVerify: true}

	session, err := quic.DialAddr(addr, &tlsConf, nil)
	if err != nil {
		panic("dialQuic")
	}

	defer session.Close()

	// open bidirectional QUIC stream => can be used to open several streams?
	stream, err := session.OpenStreamSync()
	if err != nil {
		panic("openStream")
	}

	for {
		_, err := stream.Write(buf)
		if err !=  nil {
			panic("writeStream")
		}
	}

	defer wg.Done()
}

func quic_client(threads int, addr string) {
	port := PORT
	var wg sync.WaitGroup

	for i := 0;  i < threads; i++ {
		destAddr := addr + ":" + strconv.Itoa(port)
		port++

		// increment sync primitive per thread
		wg.Add(1)
		go quic_client_worker(destAddr, &wg)
	}

	wg.Wait()
	fmt.Println("Releasing threads")
}

func quic_server_worker(c chan<- measurement, port int) {
	var bytesPerInterval uint64 = 0
	buf := make([]byte, 1400, 1400)
	listenAddr := "[::]:" + strconv.Itoa(port)

	fmt.Println("goroutine: listening on ", listenAddr)
	
	tlsConf := create_tls_config()
	// debug fmt.Println("goroutine: tls config is ", reflect.TypeOf(tlsConf))

	/* Server started with ListenAddr
	   Creates packet conn and listening on given address
	 */
	packetConn, err := quic.ListenAddr(listenAddr, tlsConf, nil)
	if err != nil {
		panic("listenAddr")
	}

	fmt.Println("goroutine: wait for incoming connection")
	// debug fmt.Println("goroutine: packet conn is ", reflect.TypeOf(packetConn))
	
	// accept incoming connection
	sess, err := packetConn.Accept()
	if err != nil {
		panic("acceptConn")
	}

	fmt.Println("goroutine: connection established")
	// debug fmt.Println("goroutine: sess is ", reflect.TypeOf(sess))

	// connection close
	// possible candidates => different granularities
	// 1) session.go: SESSION level => func (s *session) Close() err
	// 2) server.go: CONNECTION level => func (s *Server) Close()
	// 3) stream.go: STREAM level => func (s *stream) Close() err
	defer sess.Close()

	/* "return next stream opened by peer" => stream NOT streamID
	   c.f. packet connection can consist of several bidirection streams
	 */
	stream, err := sess.AcceptStream ()
	if err != nil {
		panic("acceptStream")
	}

	fmt.Println("goroutine: stream accepted")
	// debug fmt.Println("goroutine: stream id is ", reflect.TypeOf(stream))

	start := time.Now()
	
	/*
	fmt.Println("goroutine: UPDATE_INTERVAL is: ", reflect.TypeOf(UPDATE_INTERVAL))
	fmt.Println("goroutine: buf is: ", reflect.TypeOf(buf))
	fmt.Println("goroutine: bytesPerInterval is: ", reflect.TypeOf(bytesPerInterval))
	*/

	for {
		// 1. stream read out
		numBytes, err := io.ReadFull(stream, buf)
		if err != nil {
			panic("readStream")
		}
		bytesPerInterval += uint64(numBytes)

		elapsed := time.Since(start)
		// elapsed.Seconds() returns float64
		if elapsed.Seconds() > float64(UPDATE_INTERVAL) {
			// 2. make result
			result := measurement{bytes: bytesPerInterval, time: elapsed.Seconds()}

			/*
			fmt.Println("goroutine: bytesPerInterval read: ", bytesPerInterval)
			fmt.Println("goroutine: time elapsed: ", elapsed.Seconds())
			*/

			c <- result
			bytesPerInterval = 0
			start = time.Now()
		}
	}
}

func quic_server(threads int) {
	var accumulated uint64
	port := PORT
	connStats := make(chan measurement)
	
	for i := 0; i < threads; i++ {
		go quic_server_worker(connStats, port)
		port++
	}

	for {
		for i := 0; i < threads; i++ {
		recvData := <- connStats
		accumulated += recvData.bytes
		}

		mByteSec := accumulated / (1000000 * uint64(UPDATE_INTERVAL))
		fmt.Println("Throughput MByte/sec: ", mByteSec)
		accumulated = 0
	}
}

func create_tls_config() *tls.Config {
	// 1. generate KEY: generate 1024 bit key using RNG
	pKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic("generateRsa")
	}
	
	// 2. x509 CERT template
	certTemplate := x509.Certificate{SerialNumber: big.NewInt(1)}
	
	// 3. create self-signed x509 certificate => DER used for binary encoded certs
	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &pKey.PublicKey, pKey)
	if err != nil {
		panic("generateX509DER")
	}

	// 4. encode key in PEM
	pKeyPEM := pem.EncodeToMemory(&pem.Block{Type : "RSA PRIVATE KEY", Bytes : x509.MarshalPKCS1PrivateKey(pKey)})

	// 5. encode certDER in PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type : "CERTIFICATE", Bytes : certDER})

	// 6. create tls cert
	tlsCert, err := tls.X509KeyPair(certPEM, pKeyPEM)
	if err != nil {
		fmt.Println("generateTlsCert")
	}

	// 7. return tls config struct, i dont get what struct member we're addressing...
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}
}

func udp_client_worker(addr string, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 1400, 1400)
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

func udp_client(threads int, addr string) {
	port := PORT
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		listen := addr + ":" + strconv.Itoa(port)
		wg.Add(1)
		go udp_client_worker(listen, &wg)
		port += 1
	}
	wg.Wait()
}

func udp_server_worker(c chan<- measurement, port int) {

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

	message := make([]byte, BYTE_BUFFER_SIZE, BYTE_BUFFER_SIZE)

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

func udp_server(threads int) {
	c := make(chan measurement)
	port := 6666
	for i := 0; i < threads; i++ {
		go udp_server_worker(c, port)
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

func tcp_client_worker(addr string, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, BYTE_BUFFER_SIZE, BYTE_BUFFER_SIZE)
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

func tcp_client(threads int, addr string) {
	port := 6666
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		listen := addr + ":" + strconv.Itoa(port)
		wg.Add(1)
		go tcp_client_worker(listen, &wg)
		port += 1
	}
	wg.Wait()
}

func tcp_server(threads int) {
	c := make(chan measurement)
	port := 6666
	for i := 0; i < threads; i++ {
		go tcp_server_worker(c, port)
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

func tcp_server_worker(c chan<- measurement, port int) {
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
	message := make([]byte, BYTE_BUFFER_SIZE, BYTE_BUFFER_SIZE)

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
	protoPtr := flag.String("protocol", "udp", "quic, udp or tcp")
	modePtr := flag.String("mode", "server", "server (\"localhost\") or IP address ")
	threadPtr := flag.Int("threads", 1, "an int for numer of coroutines")

	flag.Parse()
	fmt.Println("trxer(c) - 2017")
	fmt.Println("Protocol:", *protoPtr)
	fmt.Println("Mode:", *modePtr)
	fmt.Println("Threads:", *threadPtr)

	if *protoPtr == "udp" {
		if *modePtr == "server" {
			udp_server(*threadPtr)
		} else {
			udp_client(*threadPtr, *modePtr)
		}
	} else if *protoPtr == "tcp" {
		if *modePtr == "server" {
			tcp_server(*threadPtr)
		} else {
			tcp_client(*threadPtr, *modePtr)
		}
	} else if *protoPtr == "quic" {
		if *modePtr == "server" {
			// server
			quic_server(*threadPtr)
		} else {
			// client
			quic_client(*threadPtr, *modePtr)
		}
	} else {
		panic("udp or tcp")
	}

}
