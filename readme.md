
Deprecated: fatun 太复杂：1. builtin没必要，2. 有状态连接没必须要，而且不好进行智能路由（针对连接进行进行其他控制应该在旁路做）





```text
              capure              uplink                      send
    process <--------> client <============> proxyer <============> forwarder <============> server
              inject             downlink                     recv
```


proxyer 到 forwarder 之间是必须是数据报，不能是有状态的，不然不好进行智能路由