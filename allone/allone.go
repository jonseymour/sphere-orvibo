package allone

import (
	"encoding/hex" // For converting stuff to and from hex
	"fmt"          // For outputting stuff
	"net"          // For networking stuff
	"os"           // For exiting
	"strings"      // For reversing strings
)

// Socket contains information about what sockets we've found
type Socket struct {
	IP          *net.UDPAddr // IP address of our socket
	State       bool         // Are we on or off?
	Name        string       // The name of the socket (e.g. "Christmas Lights")
	MACAddress  string       // The MAC address of our socket (e.g. ACCFDEADBEEF)
	Subscribed  bool         // Have we subscribed to this socket yet?
	LastMessage string       // For debugging, mostly. Last message the socket received
}

// EventStruct is what we pass back to our calling code via channels
type EventStruct struct {
	Name       string // The name of the event (e.g. ready, socketfound)
	SocketInfo Socket // And our Socket struct so we can look at IP address, MAC etc.
}

var sockets = make(map[string]*Socket) // All the sockets we've found
var twenties = "202020202020"          // This is padding for the MAC Address, so define it here for brevity
var conn *net.UDPConn                  // Our UDP connection, read and write
var msg []byte

// Events gets passed back to our calling code and acts like
// EventEmitters in node.js. Listen for a new channel item. Got one? Parse it!
var Events = make(chan EventStruct)

// PrepareSockets gets our UDP socket ready for reading and writing
func PrepareSockets() {
	if getLocalIP() == "" {
		fmt.Println("Error: Can't determine local IP address. Exiting!")
		os.Exit(1)
	} else {
		fmt.Println("Local IP is:", getLocalIP())
	}

	udpAddr, _ := net.ResolveUDPAddr("udp4", ":10000") // Get our address ready for listening
	conn, _ = net.ListenUDP("udp", udpAddr)            // Now we listen on the address we just resolved
	go func() { Events <- EventStruct{"ready", Socket{}} }()
}

func Discover() {
	broadcastMessage("686400067161")
}

func Subscribe() {
	for k := range sockets { // Loop over all sockets we know about
		//
		//

		// I know you won't read this, so here's an ugly whitespace to distract you until you fix this bug!
		if sockets[k].Subscribed == false { // If we haven't subscribed. BUG: THIS WILL FAIL WHEN SUBSCRIPTION LAPSES. JSUT SUBSCRIBE TO ALL SOCKETS ANYWAY!!
			//
			//
			//
			macReverse := reverseMAC(sockets[k].MACAddress)
			fmt.Println("Sending sub message..")
			sendMessage("6864001e636c"+sockets[k].MACAddress+twenties+macReverse+twenties, sockets[k].IP)

		}
	}
	return
}

func Query() {
	for k := range sockets { // Loop over all sockets we know about
		if sockets[k].Subscribed == true { // If we haven't subscribed
			sendMessage("6864001D7274"+sockets[k].MACAddress+twenties+"0000000004000000000000", sockets[k].IP)
		}
	}
}

func CheckForMessages() { // Now we're checking for messages
	fmt.Println("Checking for messages")
	var buf [1024]byte // We want to get 1024 bytes of messages (is this enough? Need to check!)

	go func() { // Rading from UDP blocks
		n, addr, _ := conn.ReadFromUDP(buf[0:])
		msg = buf[0:n] // Set this property so other functions can use it. n is how many bytes we grabbed from UDP

		if n > 0 && addr.IP.String() != getLocalIP() { // If we've got more than 0 bytes
			fmt.Println("Yo, Message was found:", n)
			go handleMessage(hex.EncodeToString(msg), addr) // Hand it off to our handleMessage func. We pass on the message and the address (for replying to messages)
		}

		msg = nil // Clear out our msg property so we don't run handleMessage on old data

	}() // Read from UDP connection. [0:] is slice stuff that says "shove everything in the first section of the byte and go until we've extracted all data"

}

func SetState(state bool, macAdd string) {
	sockets[macAdd].State = state
	var statebit string
	if state == true {
		statebit = "01"
	} else {
		statebit = "00"
	}

	sendMessage("686400176463"+macAdd+twenties+"00000000"+statebit, sockets[macAdd].IP)
	go func() { Events <- EventStruct{"stateset", *sockets[macAdd]} }()
}

func handleMessage(message string, addr *net.UDPAddr) {
	if len(message) == 0 {
		return
	}
	commandID := message[8:12] // What command we've received back

	macStart := strings.Index(message, "accf")
	macAdd := message[macStart:(macStart + 12)] // The MAC address of the socket responding

	fmt.Println("Message:", message, "IP:", addr.IP.String(), "MAC:", macAdd, "CID:", commandID)
	switch commandID {
	case "7161":
		_, ok := sockets[macAdd] // Check to see if we've already got macAdd in our array
		fmt.Println("Added before?", ok)
		if ok == false { // If we haven't added this socket yet
			lastBit := message[(len(message) - 1):]
			if lastBit == "00" {
				fmt.Println("Socket is off")
				sockets[macAdd] = &Socket{addr, false, "", macAdd, false, message} // Add the socket
				go func() {
					fmt.Println("Found")
					Events <- EventStruct{"socketfound", *sockets[macAdd]}
					fmt.Println("Found")
				}()
			} else {
				fmt.Println("Socket is on")
				sockets[macAdd] = &Socket{addr, true, "", macAdd, false, message} // Add the socket
				go func() {
					Events <- EventStruct{"socketfound", *sockets[macAdd]}
				}()

			}
		} else {
			Events <- EventStruct{"socketfound", *sockets[macAdd]}
		}
	case "636c":
		sockets[macAdd].Subscribed = true
		go func() {
			Events <- EventStruct{"subscribed", *sockets[macAdd]}
		}()

	case "7274": // We've queried our socket, this is the data back
		// Our name starts after the fourth 202020202020
		strName := message[140:172]
		fmt.Println("!!!!!!!!!! NAME:", strName)
		// If no name has been set, we get 32 bytes of F back, so
		// we create a generic name so our socket name won't be spaces
		// And our name is 32 bytes long.
		strDecName, _ := hex.DecodeString(strName[0:14])
		if strName[0:18] == "ffffffffffffffff" {
			fmt.Println("Blank name found")
		}
		// Convert back to text and assign
		sockets[macAdd].Name = string(strDecName)

		go func() {
			Events <- EventStruct{"queried", *sockets[macAdd]}
		}()
		break
	}
}
func sendMessage(msg string, ip *net.UDPAddr) {
	fmt.Println("Sending message:", msg, "to", ip.String())

	// Turn this hex string into bytes for sending
	buf, _ := hex.DecodeString(msg)

	// Resolve our address, ready for sending data
	udpAddr, _ := net.ResolveUDPAddr("udp4", ip.String())

	// Actually write the data and send it off
	// _ lets us ignore "declared but not used" errors. If we replace _ with n,
	// We'd have to use n somewhere (e.g. fmt.Println(n)), but _ lets us ignore that
	_, _ = conn.WriteToUDP(buf, udpAddr)

	// If we've got an error

	return

}

func broadcastMessage(msg string) {
	fmt.Println("Broadcasting message:", msg, "to", net.IPv4bcast.String()+":10000")
	udpAddr, err := net.ResolveUDPAddr("udp4", net.IPv4bcast.String()+":10000")

	buf, _ := hex.DecodeString(msg)

	// If we've got an error
	if err != nil {
		fmt.Println("ERROR!:", err)
	}
	_, _ = conn.WriteToUDP(buf, udpAddr)

}

// Okay, so this is a clusterfuck, of sorts.
// Linux will use return during the first for loop because that's how it finds addresses.
// Windows will not get a valid address from the first for and have to look differently to find the address
// I'll clean this function up when I get a chance, but for now, it's on to more coding
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("Oops: " + err.Error() + "\n")
		return ""
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	ifaces, _ := net.Interfaces()
	// handle err
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPAddr:
				return v.IP.String()
			}

		}
	}

	return ""
}

// Via http://stackoverflow.com/questions/19239449/how-do-i-reverse-an-array-in-go
// Splits up a hex string into bytes then reverses the bytes
func reverseMAC(mac string) string {
	s, _ := hex.DecodeString(mac)
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return hex.EncodeToString(s)
}
