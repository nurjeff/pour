package pour

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type logModel struct {
	Log       string      `json:"log"`
	Timestamp string      `json:"time"`
	Tag       ModelLogTag `json:"tag"`
	FileName  string      `json:"file_name"`
	FileLine  int         `json:"file_line"`
}

type concurrentSlice struct {
	sync.RWMutex
	items []logModel
}

// Do not instantiate this, instead use one of the default Tag types
// Like: pour.TagSuccess, pour.TagWarning, pour.TagError
type ModelLogTag struct {
	ID    uint   `json:"index" bson:"index"`
	Color string `json:"color" bson:"color"`
	Name  string `json:"name" bson:"name"`
}

var tags []ModelLogTag

var cache concurrentSlice = concurrentSlice{}
var localcache concurrentSlice = concurrentSlice{}
var runTime string
var logPath = "."
var useTLS = true

var errorHardwareAmount = 0
var errorLogAmount = 0

const MAX_HARDWARE_ERRORS = 2
const MAX_LOG_ERRORS = 2

func SetUseTLS(use bool) {
	useTLS = use
}

const TAG_SUCCESS = 1
const TAG_WARNING = 2
const TAG_ERROR = 3

func fillDefaultTags() {
	tags = []ModelLogTag{}
	tags = append(tags, ModelLogTag{Color: "#1c9c3e", Name: "Success", ID: 1})
	tags = append(tags, ModelLogTag{Color: "#c2a525", Name: "Warning", ID: 2})
	tags = append(tags, ModelLogTag{Color: "#9c1f1f", Name: "Error", ID: 3})
}

// Do not call this ever, this is required for depedency injection for the server
func SystemDefautTags() []ModelLogTag {
	fillDefaultTags()
	return tags
}

func LogErr(err error) {
	LogTagged(false, TAG_ERROR, err.Error())
}

func Log(args ...interface{}) {
	_, filename, line, ok := runtime.Caller(1)
	go func(filename string, line int, ok bool, args ...interface{}) {

		str := ""
		filenameLog := ""
		lineLog := 0
		if ok {
			filenameLog = filename
			lineLog = line
			lastFiles := strings.Split(filenameLog, "/")
			if len(lastFiles) > 0 {
				str += lastFiles[len(lastFiles)-1] + ":" + fmt.Sprint(lineLog) + " "
			} else {
				str += filenameLog + ":" + fmt.Sprint(lineLog) + " "
			}

		}
		for _, element := range args {
			str += fmt.Sprint(element) + " "
		}
		prnt(ColorWhite, str)
		go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
		cache.RWMutex.Lock()
		defer cache.RWMutex.Unlock()

		cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"), FileName: filenameLog, FileLine: lineLog})
	}(filename, line, ok, args)
}

func LogColor(silent bool, color string, args ...interface{}) {
	go func(args ...interface{}) {
		str := ""
		for _, element := range args {
			str += fmt.Sprint(element) + " "
		}
		if !silent {
			prnt(color, str)
		}
		go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
		cache.RWMutex.Lock()
		defer cache.RWMutex.Unlock()

		cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")})
	}(args)
}

func LogPanicKill(exitCode int, args ...interface{}) {
	str := "PANIC:"
	for _, element := range args {
		str += fmt.Sprint(element) + " "
	}
	prnt(ColorRed, str)
	go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
	cache.RWMutex.Lock()
	defer cache.RWMutex.Unlock()
	cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")})
	os.Exit(exitCode)
}

func LogTagged(silent bool, tag uint, args ...interface{}) {
	_, filename, line, ok := runtime.Caller(1)
	if ok {
		if strings.Contains(filename, "logic.go") {
			_, filename, line, ok = runtime.Caller(2)
		}
	}
	go func(tag uint, filename string, line int, ok bool, args ...interface{}) {
		if tag <= 0 || tag > uint(len(tags)) {
			tag = 1
		}

		color := ""
		switch tag {
		case TAG_ERROR:
			color = ColorRed
		case TAG_SUCCESS:
			color = ColorGreen
		case TAG_WARNING:
			color = ColorYellow
		default:
			color = ColorWhite
		}

		str := ""
		filenameLog := ""
		lineLog := 0
		if ok {
			filenameLog = filename
			lineLog = line
			lastFiles := strings.Split(filenameLog, "/")
			if len(lastFiles) > 0 {
				str += lastFiles[len(lastFiles)-1] + ":" + fmt.Sprint(lineLog) + " "
			} else {
				str += filenameLog + ":" + fmt.Sprint(lineLog) + " "
			}

		}
		for _, element := range args {
			str += fmt.Sprint(element) + " "
		}
		if !silent {
			prnt(color, str)
		}

		go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
		cache.RWMutex.Lock()
		defer cache.RWMutex.Unlock()
		cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"), Tag: tags[tag-1]})
	}(tag, filename, line, ok, args)
}

func localLog(msg string, time string) {
	localcache.RWMutex.Lock()
	defer localcache.RWMutex.Unlock()

	if runTime == "" {
		localcache.items = append(localcache.items, logModel{Log: msg, Timestamp: time})
		return
	}
	if !exists(logPath + "/logs") {
		os.Mkdir(logPath+"/logs", 0755)
	}
	f, err := os.OpenFile(logPath+"/logs/"+runTime+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	for _, elements := range localcache.items {
		if _, err := f.Write([]byte(elements.Timestamp + ":" + elements.Log + "\n")); err != nil {
			log.Fatal(err)
		}
	}
	localcache.items = []logModel{}
	if _, err := f.Write([]byte(time + ":" + msg + "\n")); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

const defaultFileContent = "{\n\t\"remote_logs\": true, \n\t\"project_key\": \"<GET THIS FROM SERVER ADMINISTRATOR>\", \n\t\"host\": \"127.0.0.1\", \n\t\"port\": 12555, \n\t\"client\": \"default_user\", \n\t\"client_key\": \"c8e0e509-ba4b-4c90-bbf2-8336627ac3ed\",\n\t\"tls\": true\n}"

type PourConfig struct {
	RemoteLogs bool   `json:"remote_logs"`
	ProjectKey string `json:"project_key"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Client     string `json:"client"`
	ClientKey  string `json:"client_key"`
	TLS        bool   `json:"tls"`
}

var config PourConfig
var loc *time.Location

// Setups up the logging connection, host and port point to the logging server, key and project build the auth required to communicate with it.
// The doRemote flag decides whether logs are sent to the remote server or are simply locally logged. isDocker is needed to distinguish between writable file paths.
func Setup(isDocker bool) {
	locRes, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		LogPanicKill(1, "Could not read location")
	}
	loc = locRes
	fillDefaultTags()
	if isDocker {
		logPath = "./data"
		if !exists("./data") {
			os.Mkdir("./data", 0755)
		}
	}

	if !exists("./config_pour.json") {
		file, err := os.Create("./config_pour.json")
		if err != nil {
			Log(false, ColorRed, "Error auto-creating pour config:", err)
			return
		}
		_, err = file.WriteString(defaultFileContent)
		if err != nil {
			Log(false, ColorRed, "Error auto-filling pour config:", err)
			return
		}

		LogPanicKill(-1, "Pour-Config ("+"./config_pour.json) was created, please fill out and restart the server")
		return
	}

	SetUseTLS(config.TLS)

	contents, err := os.ReadFile("./config_pour.json")
	if err != nil {
		LogPanicKill(-1, "Couldn't read pour config")
		return
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		LogPanicKill(-1, "Couldn't read pour config")
		return
	}

	if config.Host == "" || config.Port <= 0 || config.ProjectKey == "" || config.Client == "" || config.ClientKey == "" {
		Log(false, ColorPurple, "LogServer values invalid, falling back to local")
	}

	LogColor(false, ColorPurple, "Log-Server configured at", config.Host+":"+fmt.Sprint(config.Port))

	runTime = time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	runTime = strings.ReplaceAll(runTime, ":", "_")

	LogColor(false, ColorGreen, "Pour up and running..")
	doRemote = config.RemoteLogs
	go pollUsage()
	go logLoop(config.Host, uint(config.Port), config.ProjectKey, config.Client, config.ClientKey)
}

var doRemote = false

func pollUsage() {
	for {
		hw := HardwareUsage{}

		v, err := mem.VirtualMemory()
		if err == nil {
			hw.MemoryFree = v.Free
			hw.MemoryTotal = v.Total
			hw.MemoryUsed = v.Used
		}

		cpus, _ := cpu.Percent(time.Second*5, true)
		hw.CPUs = cpus
		stats, err := cpu.Info()
		if err == nil {
			if len(stats) >= 1 {
				hw.CPUInfo = stats[0]
			}
		}

		go sendHardwareUsage(hw)
		time.Sleep(time.Second * 30)
	}
}

func sendHardwareUsage(hw HardwareUsage) {
	b, err := json.Marshal(&hw)
	if err != nil && errorHardwareAmount < MAX_HARDWARE_ERRORS {
		errorHardwareAmount++
		Log(false, ColorRed, "Error marshalling hardware-info", err)
		return
	}
	httpPrefix := "http://"
	if useTLS {
		httpPrefix = "https://"
	}
	req, err := http.NewRequest("PATCH", httpPrefix+config.Host+":"+fmt.Sprint(config.Port)+"/logs/projects/hardware", strings.NewReader(string(b)))
	if err != nil && errorHardwareAmount < MAX_HARDWARE_ERRORS {
		//Handle Error
		errorHardwareAmount++
		Log(false, ColorRed, "Error creating hardware-info request", err)
		return
	}

	req.Header.Add("X-CLIENT", config.Client)
	req.Header.Add("Authorization", config.ClientKey)
	req.Header.Add("X-KEY", config.ProjectKey)

	res, err := client.Do(req)
	if err != nil {
		if errorHardwareAmount < MAX_HARDWARE_ERRORS {
			Log(false, ColorRed, "Error transmitting hardware-info", err)
			errorHardwareAmount++
		}
		return
	}

	if res.StatusCode != http.StatusAccepted && errorHardwareAmount < MAX_HARDWARE_ERRORS {
		errorHardwareAmount++
		Log(false, ColorRed, "Error transmitting hardware-info", res)
		return
	}
}

type HardwareUsage struct {
	MemoryTotal uint64       `json:"memory_total"`
	MemoryUsed  uint64       `json:"memory_used"`
	MemoryFree  uint64       `json:"memory_free"`
	CPUs        []float64    `json:"cpus"`
	CPUInfo     cpu.InfoStat `json:"cpu_info"`
}

func logLoop(host string, port uint, key string, client string, clientKey string) {
	for {
		time.Sleep(time.Second * 5)
		if doRemote && len(cache.items) > 0 {
			remoteLog(cache.items, host, port, key, client, clientKey)
		}
	}
}

var client = http.Client{Transport: &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // <--- Problem
}}

func remoteLog(logs []logModel, host string, port uint, key string, logClient string, clientKey string) error {
	if errorLogAmount > 10 {
		err := errors.New("unsuccessfully re-tried remote logging 10 times, disabling remote..")
		LogErr(err)
		doRemote = false
		return err
	}
	cache.RWMutex.Lock()
	defer cache.RWMutex.Unlock()
	b, err := json.Marshal(&logs)
	if err != nil && errorLogAmount < MAX_LOG_ERRORS {
		errorLogAmount++
		Log(false, ColorRed, "Error marshalling logs", err)
		return err
	}
	httpPrefix := "http://"
	if useTLS {
		httpPrefix = "https://"
	}
	req, err := http.NewRequest("POST", httpPrefix+host+":"+fmt.Sprint(port)+"/logs", strings.NewReader(string(b)))
	if err != nil && errorLogAmount < MAX_LOG_ERRORS {
		//Handle Error
		errorLogAmount++
		Log(false, ColorRed, "Error marshalling logs", err)
		return err
	}

	req.Header.Add("X-CLIENT", logClient)
	req.Header.Add("Authorization", clientKey)
	req.Header.Add("X-KEY", key)

	res, err := client.Do(req)
	if err != nil {
		if errorLogAmount < MAX_LOG_ERRORS {
			errorLogAmount++
			Log(false, ColorRed, "Error transmitting logs", err)
		}
		return err
	}
	if res.StatusCode == http.StatusAccepted {
		cache.items = []logModel{}
	} else {
		if errorLogAmount < MAX_LOG_ERRORS {
			errorLogAmount++
			defer res.Body.Close()
			read, err := io.ReadAll(res.Body)
			if err == nil {
				Log(false, ColorRed, "Error logging", string(read))
			} else {
				Log(false, ColorRed, "Error logging", res.StatusCode)
			}
		}
	}
	return nil
}

func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

const ColorReset = "\033[0m"
const ColorGreen = "\033[32m"
const ColorYellow = "\033[33m"
const ColorBlue = "\033[34m"
const ColorPurple = "\033[35m"
const ColorCyan = "\033[36m"
const ColorWhite = "\033[37m"
const ColorRed = "\033[31m"

func prnt(color string, text string) {
	fmt.Print("[" + time.Now().In(loc).Format(time.RFC822) + "] ")
	/*text := ""
	for _, element := range args {
		text += fmt.Sprint(element)
		text += " "
	}*/
	fmt.Print(string(color))
	fmt.Print(text)
	fmt.Println(ColorWhite)
}
