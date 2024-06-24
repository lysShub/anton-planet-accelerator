

当前route probe 对当上行，或者开头上行大量数据包的情况不适用,


代理模式: 
    固定节点模式: 因为proxyer支持1:N forward, 因此需要在Header增加LocID字段, proxyer会根据这个字段选择最近的forward（如果
                 proxyer有多个相同LocID的forward，这些forward应该等价）, client可能有多个proxyer（这些proxyer不应该是等价
                 的）, 在这种模式下，会用携带LocID轮流Ping各个forward，从中选出最小延时的；然后开始代理。
    
    智能路由模式: 复用固定节点模式，只不过地理标签是根据server ip获取的。



