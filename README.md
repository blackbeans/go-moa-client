#### MOA Client使用方式

* 安装：
    - 因为使用了私有仓库因此必须使用 —insecure参数
    ``` 
    go get -insecure git.wemomo.com/bibi/go-moa-client/client
    go get -insecure git.wemomo.com/bibi/go-moa/proxy
    
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

