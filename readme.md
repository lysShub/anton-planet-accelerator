1. 数据报，无连接
2. 复用
3. 包含唯一id





```text

    process <﹒﹒﹒﹒> client <============> gateway <----------------------------------> forwarder <============> server
                                                       不存在路由，地理上是1:1, 1:N的
                                                       服务器只是为了负载均衡                             
```


无状态的，只需要管理下端口，直接用IPconn

