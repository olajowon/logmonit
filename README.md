# LogMonit
***

## Introduction
LogMonit 是对**日志文件**进行实时检查、报警的Daemon服务。


## Configuration & Start-up
### 日志配置 *.toml
参考 tpl/example.toml 创建日志配置，保持一个日志一个配置，请勿出现重名。

例：tpl/loggrove.toml
	
	name="LoggroveLog"
	path="/tmp/loggrove.log"
	
	[[monitors]]
	name="Logfile"
	match="logfile"
	interval=1
	expression="1<%d"
	webhook="https://oapi.dingtalk.com/robot/send?access_token=67c8ced5356f71e4ed45579972c899c225562c850ae95a571d647239a1d7b0e1"
	comment="日志文件"
	
	[[monitors]]
	name="Total"
	match="."
	interval=5
	expression="200<%d"
	webhook="https://oapi.dingtalk.com/robot/send?access_token=67c8ced5356f71e4ed45579972c899c225562c850ae95a571d647239a1d7b0e1"
	comment="全部"

### 编译 logmonit 
	go build -o logmonit .
	
### 启动 
	./logmonit 
	
## TO DO
		
更多支持及使用说明会陆续完善...