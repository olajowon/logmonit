package main

import (
	"fmt";
	"io/ioutil"
	"time"
	"math"
	"os"
	"strings"
	"regexp"
	"os/exec"
	"github.com/apaxa-go/eval"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"github.com/BurntSushi/toml"
)

type Dps [][2]int64 // DataPoint  [[时间戳, 统计值], ...]

type MonitorDpMp map[string]Dps // 监控项DataPoint {监控项:[[时间戳, 统计值], ...], ...}

type Logfile struct {
	// 日志文件
	Name     string
	Path     string
	Find     string
	Monitors []Monitor
}

type Monitor struct {
	// 监控项
	Name       string
	Match      string
	Interval   int64
	Expression string
	Webhook    string
	Comment    string
}

const configDir = "configs/"
const logPath = "/tmp/logmonit.log"

var log = logrus.New()
var LOGFILES []Logfile
var POSITION_MAP map[string]int64        // 最后一次读取文件位置map   {日志名称:位置, ...}
var DATAPOINT_MAP map[string]MonitorDpMp // 所有日志 Datapoint存储  {日志名称: {监控项:[[时间戳, 统计值], ...], ...}, ...}

func makeLogfiles() {
	/*
		生产&更新 LOGFILES
	*/
	for {
		log.Info("makeLogfiles begin")
		var logfiles []Logfile

		rd, err := ioutil.ReadDir(configDir)
		if err != nil {
			log.Errorf("makeLogfiles error: %s", err)
			fmt.Println("makeLogfiles error: %s", err)
		} else {
			for _, fi := range rd {
				if fi.Name() == "example.toml" {
					continue
				}

				var logfile Logfile
				var configFile string
				configFile = configDir + fi.Name()
				_, err := toml.DecodeFile(configFile, &logfile)
				if err != nil {
					log.Warn("makeLogfiles warn: %s", err)
					fmt.Println("makeLogfiles warn: %s", err)
					continue
				}

				// 判断 logfile 有效性
				if logfileValid(&logfile, logfiles, configFile) {
					logfiles = append(logfiles, logfile)
				}
			}
		}
		LOGFILES = logfiles
		log.Info("makeLogfiles end")
		time.Sleep(time.Duration(30) * time.Second)
	}
}

func logfileValid(logfile *Logfile, logfiles []Logfile, configFile string) bool {
	valid := true

	if logfile.Path == "" && logfile.Find != "" {
		cmd := exec.Command("/bin/bash", "-c", logfile.Find)
		buf, err := cmd.Output()
		if err != nil {
			msg := fmt.Sprintf(
				"makeLogfiles warn: [config: %s, Find: %s] %v", configFile, logfile.Find, err)
			log.Warn(msg)
			fmt.Println(msg)
		} else {
			logfile.Path = strings.TrimSpace(string(buf))
		}
	}

	// 检查 日志路径 是否存在
	f, err := os.Stat(logfile.Path)
	if err != nil {
		msg := fmt.Sprintf(
			"makeLogfiles warn: [config: %s, Path: %s] %v", configFile, logfile.Path, err)
		fmt.Println(msg)
		log.Warn(msg)
		valid = false
	} else {
		if f.IsDir() {
			msg := fmt.Sprintf(
				"makeLogfiles warn: [config: %s, Path: %s] Path is dir", configFile, logfile.Path)
			fmt.Println(msg)
			log.Warn(msg)
			valid = false
		}
	}

	// 检查 日志名称、路径 是否重复
	for _, lf := range logfiles {
		if lf.Name == logfile.Name {
			msg := fmt.Sprintf(
				"makeLogfiles warn: [config: %s, Name: %s] Name repetitions", configFile, logfile.Name)
			fmt.Println(msg)
			log.Warn(msg)
			valid = false
		}
		if lf.Path == logfile.Path {
			msg := fmt.Sprintf(
				"makeLogfiles warn: [config: %s, Path: %s] Path repetitions", configFile, logfile.Path)
			fmt.Println(msg)
			log.Warn(msg)
			valid = false
		}
	}

	// 检查 监控项 是否存在
	if len(logfile.Monitors) == 0 {
		msg := fmt.Sprintf("makeLogfiles warn: [config: %s] No monitors", configFile)
		fmt.Println(msg)
		log.Warn(msg)
		valid = false
	}

	// 检查 监控项
	var monitorNames []string
	for _, monitor := range logfile.Monitors {
		// 检查 名称、匹配、webhook
		if monitor.Name == "" || monitor.Match == "" || monitor.Webhook == "" {
			msg := fmt.Sprintf(
				"makeLogfiles warn: [config: %s Monitor: %s] (Name, Match, Webhook) Required",
				configFile, monitor.Name)
			fmt.Println(msg)
			log.Warn(msg)
			valid = false
		}

		// 检查 监控区间 是否正确
		if monitor.Interval < 1 || monitor.Interval > 1440 {
			msg := fmt.Sprintf(
				"makeLogfiles warn: [config: %s Monitor: %s] Interval must be between 1 and 1440",
				configFile, monitor.Name)
			fmt.Println(msg)
			log.Warn(msg)
			valid = false
		}

		// 检查 表达式 是否正确
		if ok, _ := regexp.MatchString("^([0-9]+[<=])?%d([<=][1-9][0-9]*)?$", monitor.Expression); !ok {
			msg := fmt.Sprintf(
				"makeLogfiles warn: [config: %s Monitor: %s]  Expression incorrect (n<%%d, %%d<n, n<%%d<n1)",
				configFile, monitor.Name)
			fmt.Println(msg)
			log.Warn(msg)
			valid = false
		}

		// 检查 监控名称 是否重复
		for _, name := range monitorNames {
			if name == monitor.Name {
				msg := fmt.Sprintf(
					"makeLogfiles warn: [config: %s Monitor: %s] Name repetitions", configFile, monitor.Name)
				fmt.Println(msg)
				log.Warn(msg)
				valid = false
			}
		}
		monitorNames = append(monitorNames, monitor.Name)
	}
	return valid
}

func getBeginPosition(logfileName string, openPosition int64, endPosition int64) (int64) {
	/*
		综合判断，获取读取日志的开始位置
	*/
	var beginPosition int64
	previousEndPosition, previousOk := POSITION_MAP[logfileName]

	switch {
	case endPosition < openPosition: // 打开位置大于结束位置：可能日志已切换
		beginPosition = 0
	case previousOk == false: // 不存在上一次结束位置：应该是第一次启动
		beginPosition = openPosition
	case previousEndPosition > endPosition: // 上一次结束位置大于本次结束位置：可能日志已切换
		beginPosition = openPosition
	default:
		beginPosition = previousEndPosition
	}

	return beginPosition
}

func getSize(path string) int64 {
	/*
		获取文件大小
	*/
	openStat, _ := os.Stat(path)
	return openStat.Size()
}

func alertCheck(datapoints [][2]int64, lowUnix int64, expression string) (bool, int64) {
	/*
		报警检查
	*/

	// 检查区间内所有计数求和
	count := int64(0)
	for _, datapoint := range datapoints {
		if datapoint[0] >= lowUnix {
			count += datapoint[1]
		}
	}

	// 使用 eval 进行检查
	exprStr := fmt.Sprintf(expression, count)
	parseStr, err := eval.ParseString(exprStr, "")
	if err != nil {
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

func task(logfile Logfile, beginTimeUnix int64) {
	/*
		监控主任务
	*/
	logfileName := logfile.Name
	logfilePath := logfile.Path
	monitors := logfile.Monitors
	currMinUnix := int64(math.Floor(float64(beginTimeUnix/60)) * 60)
	log.Infof("func:run, begin: %s", logfileName)

	openPosition := getSize(logfilePath)                                           // 获取文件位置
	time.Sleep(time.Duration(60-int(time.Now().Unix()-currMinUnix)) * time.Second) // 等待一分钟
	endPosition := getSize(logfilePath)                                            // 获取一分钟后大小作为结束位置
	beginPosition := getBeginPosition(logfileName, openPosition, endPosition)      // 获取开始读取的位置
	log.Infof("func:run, Position: %d, %d", beginPosition, endPosition)
	POSITION_MAP[logfileName] = endPosition // 更新文件读取的最后位置

	// 读取一分钟内新写入的日志内容
	f, err := os.Open(logfilePath)
	if err != nil {
		log.Errorf("func:run, Open file error: %s", err)
		return
	}
	defer f.Close()
	f.Seek(beginPosition, 0)
	bytes := make([]byte, endPosition-beginPosition)
	br, err := f.Read(bytes)
	if err != nil {
		log.Errorf("func:run, Read error: %s", err)
		return
	}
	lines := strings.Split(strings.TrimSpace(string(bytes[:br])), "\n") // 获取行数据

	// 创建统计map
	countMap := make(map[string]int64)
	for _, monitor := range monitors {
		countMap[monitor.Name] = 0
	}

	// 检查行内容 & 更新统计map
	for _, line := range lines {
		for _, monitor := range monitors {
			if ok, _ := regexp.MatchString(monitor.Match, line); ok {
				countMap[monitor.Name] += 1
			}
		}
	}

	// 存储监控统计（只保留后1500个数）& 检查报警
	monitorDpMap := make(MonitorDpMp)
	for _, monitor := range monitors {
		count := countMap[monitor.Name]
		datapoint := [2]int64{currMinUnix, count}
		monitorName := monitor.Name
		monitorInterval := monitor.Interval
		monitorExp := monitor.Expression

		monitorDpMap[monitorName] = append(DATAPOINT_MAP[logfileName][monitorName], datapoint) // 更新存储
		if dpLen := len(monitorDpMap[monitorName]); dpLen > 1500 {
			monitorDpMap[monitorName] = monitorDpMap[monitorName][dpLen-1500:]
		}

		if (currMinUnix+60)%(monitorInterval*60) == 0 { // 判断当前时间是否应该检查报警
			lowUnix := currMinUnix - monitorInterval*60 + 60
			if alert, count := alertCheck(monitorDpMap[monitorName], lowUnix, monitorExp); alert { // 检查报警
				log.Infof("func:run, alert: %s, %s, %d", logfileName, monitorName, count)
				SendAlert(logfile, monitor, lowUnix, currMinUnix, count) // 发报警
			}
		}

	}
	DATAPOINT_MAP[logfileName] = monitorDpMap
	log.Infof("func:run, end: %s", logfileName)
}

func makeTask() {
	/*
		日志文件分配处理
	*/
	beginTimeUnix := time.Now().Unix()
	for _, logfile := range LOGFILES {
		go task(logfile, beginTimeUnix)
	}
}

func init() {
	/*
		初始化
	*/
	log.Out = os.Stdout
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err == nil {
		log.Out = file
	}
}

func main() {
	fmt.Println("LogMonit running ...")
	log.Infof("LogMonit running ...")

	POSITION_MAP = make(map[string]int64)        // {日志名称:位置, ...}
	DATAPOINT_MAP = make(map[string]MonitorDpMp) // {日志名称: {监控项:[[时间戳, 统计值], ...], ...}, ...}
	go makeLogfiles()                            // 异步生产日志文件数组

	// 定时监控任务
	c := cron.New()
	spec := "0 */1 * * * ?"
	c.AddFunc(spec, makeTask)
	c.Start()
	select {}

	log.Info("LogMonit exit ...")
	fmt.Println("LogMonit exit ...")
}