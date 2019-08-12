package main

import (
	"fmt";
	"io/ioutil"
	"time"
	"math"
	"os"
	"strings"
	"regexp"
	"github.com/apaxa-go/eval"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"github.com/BurntSushi/toml"
)

type dps [][2]int64

type itemDpMp map[string]dps

type logFile struct {
	Name string
	Path string
	Monitors []monitorItem
}

type monitorItem struct {
	Name string
	Match string
	Interval int64
	Expression string
	Webhook string
	Comment string
}

const tomldir = "tpl/"
const logpath = "/tmp/logmntr.log"
var log = logrus.New()
var LOGFILES []logFile
var POSITION_MAP map[string]int64
var DATAPOINT_MAP map[string]itemDpMp

func makeLogFiles(){
	for {
		log.Info("makeLogfiles begin")
		var logfiles []logFile

		rd, err := ioutil.ReadDir(tomldir)
		if err != nil {
			log.Errorf("makeLogFiles error: %s", err)
			time.Sleep(time.Duration(60)*time.Second)
			continue
		}else{
			for  _, fi  := range rd {
				var logfile logFile
				var tomlpath string
				tomlpath = tomldir + fi.Name()
				_, err := toml.DecodeFile(tomlpath, &logfile)
				if err != nil {
					log.Errorf("makeLogfiles error: %s", err)
					time.Sleep(time.Duration(60)*time.Second)
					continue
				}

				logfiles = append(logfiles, logfile)
			}
		}

		LOGFILES = logfiles
		log.Info("makeLogfiles end")
		time.Sleep(time.Duration(60)*time.Second)
	}
}

func getBeginPosition(logfileName string, openPosition int64, endPosition int64)(int64){
	var beginPosition int64
	previousEndPosition, previousOk := POSITION_MAP[logfileName]

	switch {
		case endPosition < openPosition :
			beginPosition = 0
		case previousOk == false:
			beginPosition = openPosition
		case previousEndPosition > endPosition:
			beginPosition = openPosition
		default:
			beginPosition = previousEndPosition
	}

	return beginPosition
}


func monitor(logfile logFile, beginTimeUnix int64){
	logfileName := logfile.Name
	logfilePath := logfile.Path
	monitorItems := logfile.Monitors
	currMinUnix := int64(math.Floor(float64(beginTimeUnix/60))*60)
	log.Infof("func:monitor, begin: %s", logfileName)

	openPosition := getSize(logfilePath)
	time.Sleep(time.Duration(60-int(time.Now().Unix()-currMinUnix))*time.Second)
	endPosition := getSize(logfilePath)
	beginPosition := getBeginPosition(logfileName, openPosition, endPosition)
	log.Infof("func:monitor, Position: %d, %d", beginPosition, endPosition)
	POSITION_MAP[logfileName] = endPosition

	f, err :=os.Open(logfilePath)
	if err != nil {
		log.Errorf("func:monitor, Open file error: %s", err)
		return
	}
	defer f.Close()
	f.Seek(beginPosition,0)
	bytes := make([]byte, endPosition-beginPosition)
	br, err := f.Read(bytes)
	if err != nil {
		log.Errorf("func:monitor, Read error: %s", err)
		return
	}
	lines := strings.Split(strings.TrimSpace(string(bytes[:br])), "\n")

	countMap := make(map[string]int64)
	for _, monitorItem := range monitorItems {
		countMap[monitorItem.Name] = 0
	}

	for _, line := range lines {
		for _, monitorItem := range monitorItems {
			if ok, _ := regexp.MatchString(monitorItem.Match, line); ok {
				countMap[monitorItem.Name] += 1
			}
		}
	}

	itemDpMap := make(itemDpMp)
	for _, monitorItem := range monitorItems {
		count := countMap[monitorItem.Name]
		datapoint := [2]int64{currMinUnix, count}
		itemName := monitorItem.Name
		itemInterval := monitorItem.Interval
		itemExp := monitorItem.Expression

		itemDpMap[itemName] = append(DATAPOINT_MAP[logfileName][itemName], datapoint)
		if dpLen := len(itemDpMap[itemName]); dpLen > 5 {
			itemDpMap[itemName] = itemDpMap[itemName][dpLen-5:]
		}

		if (currMinUnix+60)%(itemInterval*60) == 0 {
			lowUnix := currMinUnix - itemInterval * 60 + 60
			if alert, count := alertCheck(itemDpMap[itemName], lowUnix, itemExp); alert {
				log.Infof("func:monitor, alert: %s, %s, %d", logfileName, itemName, count)
				SendAlert(logfile, monitorItem, lowUnix, currMinUnix, count)
			}
		}

	}
	DATAPOINT_MAP[logfileName] = itemDpMap
	log.Infof("func:monitor, end: %s", logfileName)
}

func getSize(path string)int64{
	openStat, _ := os.Stat(path)
	return openStat.Size()
}

func alertCheck(datapoints [][2]int64, lowUnix int64, expression string)(bool, int64){
	count := int64(0)
	for _, datapoint := range datapoints {
		if datapoint[0] >= lowUnix {
			count += datapoint[1]
		}
	}
	exprStr := fmt.Sprintf(expression, count)
	parseStr, err := eval.ParseString(exprStr,"")
	if err != nil{
		log.Errorf("func:alertCheck, ParseString error: %s", err)
		return false, count
	}
	res, err := parseStr.EvalToInterface(nil)
	if err != nil {
		log.Errorf("func:alertCheck, EvalToInterface error: %s", err)
		return false, count
	}

	if fmt.Sprintf("%v", res) == "true" {
		return true, count
	} else {
		return false, count
	}
}


func monitorBasic(){
	beginTimeUnix := time.Now().Unix()
	for _, logfile := range LOGFILES {
		go monitor(logfile, beginTimeUnix)
	}
}

func init(){
	log.Out = os.Stdout
	file, err := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY, 0666)
	if err == nil {
		log.Out = file
	}
}

func main(){
	fmt.Println("LogMonitor running ...")
	log.Infof("LogMonitor running ...")

	POSITION_MAP = make(map[string]int64)
	DATAPOINT_MAP = make(map[string]itemDpMp)
	go makeLogFiles()

	c := cron.New()
	spec := "0 */1 * * * ?"
	c.AddFunc(spec, monitorBasic)
	c.Start()
	select{}

	log.Info("LogMonitor exit ...")
	fmt.Println("LogMonitor exit ...")
}