package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/goburrow/modbus"
	sma "github.com/manios/sma-webbox-go"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const INVERTER2IPADDRESS = "192.168.10.41:502"
const INVERTER3IPADDRESS = "192.168.10.42:502"
const INVERTER4IPADDRESS = "192.168.10.52:502"

// From the document MODBUS-HTML_SB30-50-1AV-40_V10/modbuslist_en.html downloaded in a ZIP from the SMA Website
// This should be in /Users/ianaber/Documents/House/Solar/SMA Inverter/Documentation/Sunny Boy/Modbus Register.html

const CurrentA = 30769
const VoltageA = 30771
const PowerA = 30773
const CurrentB = 30957
const VoltageB = 30959
const PowerB = 30961
const VoltageC = 30963
const CurrentC = 30965
const PowerC = 30967

type InverterString struct {
	Power   float64 `json:"watts"`
	Voltage float64 `json:"volts"`
	Current float64 `json:"amps"`
}

type CurrentValues struct {
	InverterStrings [11]InverterString `json:"strings"`
	TotalPower      float64            `json:"total"`
	mu              sync.Mutex
}

var (
	panels     CurrentValues
	verbose    bool
	dbLogin    string
	dbPassword string
	dbDatabase string
	dbServer   string
)

func init() {
	var err error = nil
	flag.BoolVar(&verbose, "v", false, "Verbose. Display data to the console as it is read")

	flag.Parse()
	if err != nil {
		log.Fatal(err)
	}
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

//type Metric struct {
//	InverterName string  `json:"inverterName"`
//	StringName   string  `json:"stringName"`
//	CurrentVal   float64 `json:"currentVal"`
//	VoltageVal   float64 `json:"voltageVal"`
//	PowerVal     float64 `json:"powerVal"`
//}

func byteToFloat64(b []byte) float64 {
	var val float64 = 0
	for i := 0; i <= 3; i++ {
		val = (val * 256) + float64(b[i])
	}
	return val
}

type SMAValue struct {
	name  string
	value string
	unit  string
}

type SMAChannelMap map[string]SMAValue

func getChannelsAsMap(channels []sma.Channel) (m SMAChannelMap) {

	m = make(SMAChannelMap)

	for i := 0; i < len(channels); i++ {
		m[channels[i].Meta] = SMAValue{channels[i].Name, channels[i].Value, channels[i].Unit}
	}
	return m
}

func stringToFloat64(s string) (f float64) {
	f = 0.0
	if len(s) == 0 {
		return
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Print(err)
	}
	return
}

func GetWebBoxData(client sma.WebboxClient, deviceKey string, ssA *InverterString, ssB *InverterString) {
	// Get the current data for the given device
	processData, err := client.GetProcessData(deviceKey)

	if err != nil {
		log.Print(err)
		if verbose {
			fmt.Print(err)
		}
		return
	} else {
		channelMap := getChannelsAsMap(processData.Result.Devices[0].Channels)
		panels.mu.Lock()
		defer panels.mu.Unlock()
		ssA.Current = stringToFloat64(channelMap["A.Ms.Amp"].value)
		ssA.Voltage = stringToFloat64(channelMap["A.Ms.Vol"].value)
		ssA.Power = stringToFloat64(channelMap["A.Ms.Watt"].value)
		ssB.Current = stringToFloat64(channelMap["B.Ms.Amp"].value)
		ssB.Voltage = stringToFloat64(channelMap["B.Ms.Vol"].value)
		ssB.Power = stringToFloat64(channelMap["B.Ms.Watt"].value)
	}
}

func readRegister(inverter modbus.Client, modbusRegister uint16, registerSize uint16, resultPrecision int) (float64, error) {
	results, err := inverter.ReadInputRegisters(modbusRegister, registerSize)
	if err != nil {
		log.Print(err)
		return 0.0, err
	}
	val := byteToFloat64(results)
	if val > 1000000 {
		return 0.0, nil
	}
	return val / math.Pow10(resultPrecision), nil
}

func recordData(db *sql.DB) {
	panels.mu.Lock()
	defer panels.mu.Unlock()

	result, err := db.Exec(`INSERT INTO logging.solar_production(amps_a, volts_a, watts_a, amps_b, volts_b, watts_b, amps_c, volts_c, watts_c, amps_d, volts_d, watts_d, amps_e, volts_e, watts_e, amps_f, volts_f, watts_f, amps_g, volts_g, watts_g, amps_h, volts_h, watts_h, amps_i, volts_i, watts_i, amps_j, volts_j, watts_j, amps_k, volts_k, watts_k)
	VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);`,
		panels.InverterStrings[0].Current, panels.InverterStrings[0].Voltage, panels.InverterStrings[0].Power,
		panels.InverterStrings[1].Current, panels.InverterStrings[1].Voltage, panels.InverterStrings[1].Power,
		panels.InverterStrings[2].Current, panels.InverterStrings[2].Voltage, panels.InverterStrings[2].Power,
		panels.InverterStrings[3].Current, panels.InverterStrings[3].Voltage, panels.InverterStrings[3].Power,
		panels.InverterStrings[4].Current, panels.InverterStrings[4].Voltage, panels.InverterStrings[4].Power,
		panels.InverterStrings[5].Current, panels.InverterStrings[5].Voltage, panels.InverterStrings[5].Power,
		panels.InverterStrings[6].Current, panels.InverterStrings[6].Voltage, panels.InverterStrings[6].Power,
		panels.InverterStrings[7].Current, panels.InverterStrings[7].Voltage, panels.InverterStrings[7].Power,
		panels.InverterStrings[8].Current, panels.InverterStrings[8].Voltage, panels.InverterStrings[8].Power,
		panels.InverterStrings[9].Current, panels.InverterStrings[9].Voltage, panels.InverterStrings[9].Power,
		panels.InverterStrings[10].Current, panels.InverterStrings[10].Voltage, panels.InverterStrings[10].Power,
	)

	if err != nil {
		log.Print(err)
		if verbose {
			fmt.Print(err)
		}
		return
	}
	rows, _ := result.RowsAffected()
	if verbose {
		fmt.Printf("%d rows written\n", rows)
	}
}

//func printValues(ss InverterString) {
//	if verbose {
//		fmt.Printf("%5.3f : %5.2f : %5.0f", ss.Current, ss.Voltage, ss.Power)
//	}
//}

func connectToInverter(address string) modbus.Client {
	client1Handler := modbus.NewTCPClientHandler(address)
	client1Handler.SlaveId = 3
	handlerErr := client1Handler.Connect()
	defer func() {
		if err := client1Handler.Close(); err != nil {
			log.Print(err)
		}
	}()
	if handlerErr != nil {
		log.Print(handlerErr)
		return nil
	}
	return modbus.NewClient(client1Handler)
}

func readInverter(client modbus.Client, StringID uint8) (amps float64, volts float64, watts float64, err error) {
	amps = 0
	volts = 0
	watts = 0
	var ampsRegister uint16
	var voltsRegister uint16
	var wattsRegister uint16
	switch StringID {
	case 'A':
		ampsRegister = CurrentA
		voltsRegister = VoltageA
		wattsRegister = PowerA
	case 'B':
		ampsRegister = CurrentB
		voltsRegister = VoltageB
		wattsRegister = PowerB
	case 'C':
		ampsRegister = CurrentC
		voltsRegister = VoltageC
		wattsRegister = PowerC
	}
	amps, err = readRegister(client, voltsRegister, 2, 3)
	if err != nil {
		log.Println(err)

		return
	}
	volts, err = readRegister(client, ampsRegister, 2, 2)
	if err != nil {
		log.Println(err)
		return
	}
	watts, err = readRegister(client, wattsRegister, 2, 0)
	if err != nil {
		log.Println(err)
		return
	}
	return
}

func setPanelData(panel int, Amps float64, Volts float64, Watts float64) {
	panels.mu.Lock()
	defer panels.mu.Unlock()

	panels.InverterStrings[panel].Current = Amps
	panels.InverterStrings[panel].Voltage = Volts
	panels.InverterStrings[panel].Power = Watts
}

func getPanelData(client modbus.Client, panelOffset int) error {
	if amps, volts, watts, err := readInverter(client, 'A'); err != nil {
		return err
	} else {
		setPanelData(panelOffset, amps, volts, watts)
	}
	if amps, volts, watts, err := readInverter(client, 'B'); err != nil {
		return err
	} else {
		setPanelData(panelOffset+1, amps, volts, watts)
	}
	if amps, volts, watts, err := readInverter(client, 'C'); err != nil {
		return err
	} else {
		setPanelData(panelOffset+2, amps, volts, watts)
	}
	return nil
}

func processLoop() {
	var webBoxDevice int32
	var client1 modbus.Client
	var client2 modbus.Client
	var client3 modbus.Client

	if verbose {
		fmt.Println("SMA Watch monitors the Sunny Boy inverters at Cedar Technology")
	}

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", dbLogin, dbPassword, dbServer, dbDatabase))
	// defer the close till after the main function has finished executing
	defer func() {
		if err := db.Close(); err != nil {
			log.Print(err)
		}
	}()

	// if there is an error opening the connection, handle it
	if err != nil {
		log.Println(err.Error())
	}

	// Open doesn't open a connection. Validate DSN data:
	err = db.Ping()
	if err != nil {
		log.Println("Error pinging the database - ", err.Error()) // proper error handling instead of panic in your app
	}

	smaClient := sma.NewWebboxClient("http://192.168.10.22:80")
	var devicesResponse sma.DevicesResponse

	devicesResponse, err = smaClient.GetDevices()

	if err != nil {
		log.Print(err)
		return
	} else {
		// Find the WRTU Inverter
		for webBoxDevice = 0; webBoxDevice < devicesResponse.Result.TotalDevicesReturned; webBoxDevice++ {
			if strings.Contains(devicesResponse.Result.Devices[webBoxDevice].Name, "WRTU") {
				break
			}
		}
		if webBoxDevice >= devicesResponse.Result.TotalDevicesReturned {
			log.Print("No string inverter found")
		}
	}

	client1Handler := modbus.NewTCPClientHandler("192.168.10.41:502")
	client1Handler.SlaveId = 3
	handlerErr := client1Handler.Connect()
	defer func() {
		if err := client1Handler.Close(); err != nil {
			log.Print(err)
		}
	}()
	if handlerErr != nil {
		log.Print(handlerErr)
		//		return
	} else {
		client1 = modbus.NewClient(client1Handler)
	}

	loops := 0 // We can only poll the WEB Box once every 15 seconds
	ticker := time.NewTicker(time.Second)

	for range ticker.C {
		if client1 == nil {
			client1 = connectToInverter(INVERTER2IPADDRESS)
		}
		if client1 != nil {
			err = getPanelData(client1, 0)
			if err != nil {
				client1 = nil
			}
		}

		if client2 == nil {
			client2 = connectToInverter(INVERTER3IPADDRESS)
		}
		if client2 != nil {
			err = getPanelData(client2, 3)
			if err != nil {
				client2 = nil
			}
		}

		if client3 == nil {
			client3 = connectToInverter(INVERTER4IPADDRESS)
		}
		if client3 != nil {
			err = getPanelData(client3, 6)
			if err != nil {
				client3 = nil
			}
		}

		loops++
		if loops == 15 {
			loops = 0
			GetWebBoxData(smaClient, devicesResponse.Result.Devices[webBoxDevice].Key, &panels.InverterStrings[9], &panels.InverterStrings[10])
			recordData(db)
		} else {
			if verbose {
				fmt.Println("")
			}
		}
	}
}

func init() {
	var port int

	flag.IntVar(&port, "port", 8081, "Port number for the WEB interface")
	flag.StringVar(&dbLogin, "dblogin", "logger", "Database login id")
	flag.StringVar(&dbPassword, "dbpwd", "logger", "Database login password")
	flag.StringVar(&dbDatabase, "Database Name", "logging", "Name of the database")
	flag.StringVar(&dbServer, "dbserver", "localhost:3306", "Database server and port")
	flag.Parse()

	log.SetFlags(log.Lshortfile | log.LstdFlags)

	go setUpWebSite(port)
}

func main() {
	running := false
	for {
		if running {
			log.Print("Restarting after error")
		} else {
			log.Print("Starting up")
		}
		running = true
		processLoop()
	}
}
