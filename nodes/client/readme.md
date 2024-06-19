

当前route probe 对当上行，或者开头上行大量数据包的情况不适用,


代理模式：
    固定节点模式：因为proxyer支持1:N forward, 因此需要在Header增加地理标签字段, 当这个字段设置时，proxyer只会发送给对应的标签的forward；client可能有多个proxyer, 在这种模式下，会用携带标签的Ping轮流ping 各个proxyer，从中选出最小延时的；然后开始代理。
    
    智能路由模式：复用固定节点模式，只不过地理标签是根据server ip获取的
    
    


