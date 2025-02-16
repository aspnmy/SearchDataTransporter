# 教程：Cherry Studio + Ollama 本地部署 + SearchDataTransporte搜索中间件服务实现联网搜索

1、本地部署Ollama，拉取对应的模型启动运行
2、测试本地Ollama api是否正常
3、Cherry Studio 配置Ollama访问
4、git本项目 git clone https://github.com/aspnmy/SearchDataTransporter.git 按照项目先部署 搜索中间件的服务端SearchDataServer
5、访问SearchDataServer端 端口查询 api联网查询接口是否正常访问 http://localhost:8080/search?q=<wd>
访问SearchDataServer端 的kafka 后端是否访问正常 http://localhost:9092
6、按项目说明部署OllamaSearchClient服务，这个可以独立部署调取Ollama的API，也可以放在Ollama的插件目录里
7、安装Cherry Studio的dev_server分支，该分支已经对Cherry Studio代码进行调整原生支持语言对话前查询SearchDataServer端实现远程联网
8、Cherry Studio 官方版本配置，则只能通过Ollama插件进行联网操作，即必须OllamaSearchClient服务能正常访问SearchDataServer端，并且能和Ollama 自身API对接成功
