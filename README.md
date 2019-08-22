# LogMonit
***

## Introduction
LogMonit 是对**日志文件**进行正则关键词检查、报警的Daemon服务，其用法简单，只需添加相应的配置文件便可完成对日志的监控。

**匹配 (match)** 支持正则表达式匹配并统计日志行数

**调度 (interval)** 支持定义报警检查区间（每n分钟检查一次），分钟为单位

**表达式 (expression)** 支持 n<%d、%d<n、n1<%d<n2 等比较运算表达式

**钩子 (webhook)** 默认支持钉钉webhook进行报警通知


## Configuration & Start-up
### 日志配置 *.toml
参考 configs/example.toml 创建日志配置，保持一个日志一个配置，请勿出现重名。

例：configs/loggrove.toml
	
	name="LoggroveLog"
	path="/tmp/loggrove.log"
	
	[[monitors]]
	name="Logfile"
	match="logfile"
	interval=1
	expression="1<%d"
	webhook="https://oapi.dingtalk.com/robot/send?access_token=xxx"
	comment="日志文件"
	
	[[monitors]]
	name="Total"
	match="."
	interval=5
	expression="200<%d"
	webhook="https://oapi.dingtalk.com/robot/send?access_token=xxx"
	comment="全部"
	
**monitors.Logfile:** 1分钟内（interval=1），匹配 "logfile"（match="logfile"） 的行数大于1时（expression="1<%d" ）则向webook (https://oapi.dingtalk.com/robot/send?access_token=xxx) 地址发送报警

**monitors.Total:** 5分钟内（interval=1），匹配 "."（match="one"） 的行数大于200时（expression="1<%d" ）则向webook (https://oapi.dingtalk.com/robot/send?access_token=xxx) 地址发送报警

### 编译 logmonit 
	go build -o logmonit .
	
### 启动 
	./logmonit