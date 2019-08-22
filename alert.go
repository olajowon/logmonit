package main

import (
	"fmt"
	"time"
	"encoding/json"
	"bytes"
	"net/http"
	"io/ioutil"
)

func SendAlert(logfile Logfile, monitor Monitor, from int64, to int64, count int64){
	/*
		发送报警
	*/
	webhook := monitor.Webhook
	logfileName := logfile.Name
	monitorName := monitor.Name
	monitorComment := monitor.Comment
	fromTime := time.Unix(from,0).Format("2006-01-02 15:04")
	toTime := time.Unix(to,0).Format("2006-01-02 15:04")
	content := fmt.Sprintf("- Logmonit 报警 -\n日志：%s\n监控：%s\n统计：%d\n时间：%s:00 - %s:59\n备注：%s\n\n",
		logfileName, monitorName, count, fromTime, toTime, monitorComment)

	data := makeData(content)
	jsonStr, err := json.Marshal(data)
	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("func:SendAlert, client.Do error: %s", err)
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	body, _ := ioutil.ReadAll(resp.Body)
	if statusCode != 200 {
		log.Errorf("func:SendAlert, resp error: %d, %s", statusCode, body)
	}
}

func makeData(content string) map[string]interface{}{
	/*
		生成 post data， 默认生成钉钉 webhook 需要的 data 格式
	*/
	data := make(map[string]interface{})
	text := make(map[string]string)
	at := make(map[string]interface{})
	text["content"] = content
	at["isAtAll"] = true
	data["msgtype"] = "text"
	data["text"] = text
	data["at"] = at
	return data
}