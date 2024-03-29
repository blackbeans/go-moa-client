#### MOA Client使用方式

* 安装：
     安装ZooKeeper (或者使用本地文件指定连接特定Server)
        $Zookeeper/bin/zkServer.sh start

    ``` 
    go get  github.com/blackbeans/go-moa-client
    
    ```

* 定义服务的接口对应的struct
    - 例如接口为：

    ```goalng
        //接口
        type IGoMoaDemo interface {
            GetDemoName(serviceUri, proto string) (DemoResult, error)
        }  
        转换为如下结构体
        type GoMoaDemo struct {
                GetDemoName func(serviceUri, proto string) (CDemoResult, error)
        }
    ```

* 客户端启动：

     ```goalng
        consumer := client.NewMoaConsumer("go_moa_client.toml",
            []proxy.Service{proxy.Service{
                ServiceUri: "/service/bibi/go-moa",
                Interface:   &GoMoaDemo{}},
        })
        //获取调用实例
        h := consumer.GetService("/service/bibi/go-moa").(*GoMoaDemo)

        for i := 0; i < 10000; i++ {
            a, err := h.GetDemoName("/service/user-profile", "redis")

            fmt.Printf("GetDemoName|%s|%v\n", a, err)
        }
    ```

* 说明
    - Service为一个服务单元，对应了远程调用的服务名称、以及对应的接口

    - MoaConsumer需要对应的Moa的配置文件，toml类型，具体配置参见conf/moa_client.toml
    
* Benchmark

    env:Macbook Pro 2.2 GHz Intel Core i7

```golang

    go test --bench=".*" github.com/blackbeans/go-moa-client/client -run=BenchmarkMakeRpcFunc

    BenchmarkParallerMakeRpcFunc-8   	   50000	     28315 ns/op

```



