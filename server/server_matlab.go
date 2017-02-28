package main

import (
	"flag"
	"fmt"
	"github.com/4180122/distbayes/distmlMatlab"
	"github.com/arcaneiceman/GoVector/govec"
	"net"
	"os"
)

const BUFFSIZE = 200000

//10485760

var (
	cnum      int = 0
	maxnode   int = 0
	myaddr    *net.TCPAddr
	cnumhist  map[int]int
	client    map[string]int
	claddr    map[int]*net.TCPAddr
	tempmodel map[int]aggregate
	testqueue map[int]map[int]bool
	models    map[int]distmlMatlab.MatModel
	modelR    map[int]map[int]float64
	modelC    map[int]float64
	modelD    float64
	channel   chan message
	logger    *govec.GoLog
	l         *net.TCPListener
	gmodel    distmlMatlab.MatGlobalModel
	gempty    distmlMatlab.MatGlobalModel
)

type aggregate struct {
	cnum  int
	model distmlMatlab.MatModel
	r     map[int]float64
	c     float64
	d     float64
}

type message struct {
	Id     int
	Ip     string
	Name   string
	Type   string
	Model  distmlMatlab.MatModel
	GModel distmlMatlab.MatGlobalModel
}

func main() {
	//Initialize stuff
	client = make(map[string]int)
	claddr = make(map[int]*net.TCPAddr)
	models = make(map[int]distmlMatlab.MatModel)
	modelR = make(map[int]map[int]float64)
	modelC = make(map[int]float64)
	modelD = 0.0
	gmodel = distmlMatlab.MatGlobalModel{nil}
	tempmodel = make(map[int]aggregate)
	testqueue = make(map[int]map[int]bool)
	cnumhist = make(map[int]int)
	channel = make(chan message)

	go updateGlobal(channel)

	//Parsing inputargs
	parseArgs()

	//Initialize TCP Connection and listener
	l, _ = net.ListenTCP("tcp", myaddr)
	fmt.Printf("Server initialized.\n")

	for {
		conn, err := l.AcceptTCP()
		checkError(err)
		go connHandler(conn)
	}

}

func connHandler(conn *net.TCPConn) {
	var msg message
	p := make([]byte, BUFFSIZE)
	conn.Read(p)
	logger.UnpackReceive("Received message", p, &msg)
	switch msg.Type {
	case "commit_request":
		//node is sending a model, must forward to others for testing
		flag := checkQueue(client[msg.Name])
		fmt.Printf("<-- Received commit request from %v.\n", msg.Name)
		if flag {
			conn.Write([]byte("OK"))
			conn.Close()
			processTestRequest(msg)
		} else {
			//denied
			conn.Write([]byte("Pending tests are not complete."))
			fmt.Printf("--> Denied commit request from %v.\n", msg.Name)
			conn.Close()
		}
	case "global_request":
		//node is requesting the global model, will forward
		conn.Write([]byte("OK"))
		fmt.Printf("<-- Received global model request from %v.\n", msg.Name)
		genGlobalModel() //TODO maybe move this elsewhere
		sendGlobal(msg)
		conn.Close()
	case "test_complete":
		//node is submitting test results, will update its queue
		conn.Write([]byte("OK"))
		fmt.Printf("<-- Received completed test results from %v.\n", msg.Name)
		testqueue[client[msg.Name]][cnumhist[msg.Id]] = false
		conn.Close()
		//update the pending commit and merge if complete
		channel <- msg
	case "join_request":
		conn.Write([]byte("OK"))
		processJoin(msg)
		conn.Close()
	default:
		fmt.Println("something weird happened!")
		fmt.Println(msg)
	}

}

func updateGlobal(ch chan message) {
	// Function that aggregates the global model and commits when ready
	for {
		m := <-ch
		id := cnumhist[m.Id]
		tempAggregate := tempmodel[id]
		tempAggregate.d += m.Model.Size
		tempAggregate.r[client[m.Name]] = m.Model.Weight
		tempmodel[id] = tempAggregate
		if modelD < tempAggregate.d {
			modelD = tempAggregate.d
		}
		if float64(tempAggregate.d) > float64(modelD)*0.6 {
			models[id] = tempAggregate.model
			modelR[id] = tempAggregate.r
			modelC[id] = tempAggregate.c
			logger.LogLocalEvent("commit_complete")
			fmt.Printf("--- Committed model%v for commit number: %v.\n", id, tempAggregate.cnum)
		}
	}
}

func genGlobalModel() {
	modelstemp := models
	modelRtemp := modelR
	modelCtemp := modelC
	modelDtemp := modelD
	gmodel = distmlMatlab.CompactGlobal(modelstemp, modelRtemp, modelCtemp, modelDtemp)
}

func processTestRequest(m message) {
	cnum++
	//initialize new aggregate
	tempweight := make(map[int]float64)
	r := m.Model.Weight
	d := m.Model.Size
	m.Model.Weight = 0.0
	m.Model.Size = 0.0
	tempweight[client[m.Name]] = r
	tempmodel[client[m.Name]] = aggregate{cnum, m.Model, tempweight, d, d}
	cnumhist[cnum] = client[m.Name]
	for name, id := range client {
		if id != client[m.Name] {
			sendTestRequest(name, id, cnum, m.Model)
		}
	}
}

func sendTestRequest(name string, id, tcnum int, tmodel distmlMatlab.MatModel) {
	//func sendTestRequest(name string, id, tcnum int, tmodel bclass.Model) {
	//create test request
	msg := message{tcnum, myaddr.String(), "server", "test_request", tmodel, gempty}
	//increment tests in queue for that node
	if queue, ok := testqueue[id]; !ok {
		queue := make(map[int]bool)
		queue[cnumhist[tcnum]] = true
		testqueue[id] = queue
	} else {
		queue[cnumhist[tcnum]] = true
	}
	//send the request
	fmt.Printf("--> Sending test request from %v to %v.", cnumhist[tcnum], name)
	err := tcpSend(claddr[id], msg)
	if err != nil {
		fmt.Printf(" [NO!]\n*** Could not send test request to %v.\n", name)
	}
}

func sendGlobal(m message) {
	fmt.Printf("--> Sending global model to %v.", m.Name)
	msg := message{m.Id, myaddr.String(), "server", "global_grant", m.Model, gmodel}
	tcpSend(claddr[client[m.Name]], msg)
}

func tcpSend(addr *net.TCPAddr, msg message) error {
	p := make([]byte, BUFFSIZE)
	conn, err := net.DialTCP("tcp", nil, addr)
	//checkError(err)
	if err == nil {
		outbuf := logger.PrepareSend(msg.Type, msg)
		_, err = conn.Write(outbuf)
		//checkError(err)
		n, _ := conn.Read(p)
		if string(p[:n]) != "OK" {
			fmt.Printf(" [NO!]\n<-- Request was denied by node: %v.\nEnter command: ", string(p[:n]))
		} else {
			fmt.Printf(" [OK]\n")
		}
	}
	return err
}

func checkQueue(id int) bool {
	flag := true
	for _, v := range testqueue[id] {
		if flag && v {
			flag = false
		}
	}
	return flag
}

func processJoin(m message) {
	//process depending on if it is a new node or a returning one
	if _, ok := client[m.Name]; !ok {
		//adding a node that has never been added before
		id := maxnode
		maxnode++
		client[m.Name] = id
		claddr[id], _ = net.ResolveTCPAddr("tcp", m.Ip)
		fmt.Printf("--- Added %v as node%v.\n", m.Name, id)
		for _, v := range tempmodel {
			sendTestRequest(m.Name, id, v.cnum, v.model)
			//sendTestRequest(m.Name, id, v.cnum, v.model)
		}
	} else {
		//node is rejoining, update address and resend the unfinished test requests
		id := client[m.Name]
		claddr[id], _ = net.ResolveTCPAddr("tcp", m.Ip)
		fmt.Printf("--- %v at node%v is back online.\n", m.Name, id)
		for k, v := range testqueue[id] {
			if v {
				aggregate := tempmodel[k]
				sendTestRequest(m.Name, id, aggregate.cnum, aggregate.model)
				//sendTestRequest(m.Name, id, aggregate.cnum, aggregate.model)
			}
		}
	}
}

func parseArgs() {
	flag.Parse()
	inputargs := flag.Args()
	var err error
	if len(inputargs) < 1 {
		fmt.Printf("Not enough inputs.\n")
		return
	}
	myaddr, err = net.ResolveTCPAddr("tcp", inputargs[0])
	checkError(err)
	logger = govec.Initialize(inputargs[1], inputargs[1])
}

//func getNodeAddr(slavefile string) {
//	dat, err := ioutil.ReadFile(slavefile)
//	checkError(err)
//	nodestr := strings.Split(string(dat), "\n")
//	for i := 0; i < len(nodestr)-1; i++ {
//		nodeaddr := strings.Split(nodestr[i], " ")
//		client[nodeaddr[0]] = i
//		claddr[i], _ = net.ResolveTCPAddr("tcp", nodeaddr[1])
//		//testqueue[i] = 0
//	}
//}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}
