1. 数据报，无连接
2. 复用
3. 包含唯一id





```text
              capure              uplink                      send
    process <--------> client <============> proxyer <============> forwarder <============> server
              inject             downlink                     recv
```


无状态的，只需要管理下端口，直接用IPconn

