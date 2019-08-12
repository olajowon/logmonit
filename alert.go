package main

import (
	"fmt"
	"time"
	"encoding/json"
	"bytes"
	"net/http"
	"io/ioutil"
)

func SendAlert(logfile logFile, monitorItem monitorItem, from int64, to int64, count int64){
	webhook := monitorItem.Webhook
	logfileName := logfile.Name
	itemName := monitorItem.Name
	fromTime := time.Unix(from,0).Format("2006-01-02 15:04")
	toTime := time.Unix(to,0).Format("2006-01-02 15:04")
	content := fmt.Sprintf("- Logmonit 报警 -\n日志：%s\n监控：%s\n统计：%d\n时间：%s:00 - %s:59\n",
		logfileName, itemName, count, fromTime, toTime)

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
	statuscode := resp.StatusCode
	body, _ := ioutil.ReadAll(resp.Body)
	if statuscode != 200 {
		log.Errorf("func:SendAlert, resp error: %d, %s", statuscode, body)
	}
}

func makeData(content string) map[string]interface{}{
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