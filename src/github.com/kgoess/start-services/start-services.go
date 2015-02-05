package main

import (
        "fmt"
        "log"
        "gopkg.in/yaml.v2"
)

var yamlStr = `
---
task1:
  cmd:
    - /bin/echo
    - task 1 is running
  descr: this task just runs echo
task2:
  cmd:
    - /bin/sleep
    - 3
  descr: sleeps for a couple secs
task3:
  after: 
  cmd:
    - /bin/ls
    - adsfadfasdf
  descr: this is expected to fail
task4:
  after:
     - task1
     - task2
  cmd:
    - /bin/echo
    - 99
  descr: this task goes after 1 and 2
`
type TaskConfig struct {
    Name string
    After []string
    Cmd []string
    Descr string
}
//type Configs struct {
//    Items []TaskConfig 
//}

func main(){
    configs, afters := loadConfigs()

    showConfigs(configs, afters)
}

func loadConfigs() (configs map[string]TaskConfig, afters map[string][]TaskConfig) {

    configs = map[string]TaskConfig{}

    err := yaml.Unmarshal([]byte(yamlStr), &configs)
    if err != nil {
        log.Fatalf("error: %v", err)
    }

    afters = map[string][]TaskConfig{}

    for taskName, task := range configs {
        task.Name = taskName
        slice := make([]TaskConfig, 5, 5) // lenght of x with room for y more

        for _, after := range task.After {
            afters[after] = slice
            afters[after][0] = task
        }
    }

    return configs, afters
}
func showConfigs(configs map[string]TaskConfig, afters map[string][]TaskConfig){
    for taskName, task := range configs {
        fmt.Printf("task:  %v (run after: %v)\ndescr: %v\n\t%v\n",
             taskName, task.After, task.Descr, task.Cmd)
        afterList, exists := afters[taskName]
        if exists {
            for _, after := range afterList {
                if after.Name != ""  {
                    fmt.Printf("after this we'll run: %s\n", after.Name)
                }
            }
        }
        fmt.Printf("------------------\n")
    }
}
/*
func main(){
    //configs := map[string]taskConfig{}
    println("hi, mom!")

    configs := make(map[interface{}]interface{})

    err := yaml.Unmarshal([]byte(data), &configs)
    if err != nil {
        log.Fatalf("error: %v", err)
    }

    fmt.Printf("--- configs are:\n%v\n\n", configs)
    task := configs["task3"]

    //type cast the interface into a map
    m, errThing := task.(map[string]string) 
    if errThing {
fmt.Printf("this is the failed thing %v\n", m)
        log.Fatalf("Can not convert value to map")
    }
    fmt.Printf("here is m: %v \n", m)

    fmt.Printf("descr is %v\n", m["descr"])
//
//    fmt.Printf("--- task4:\n%v\n\n", task)
//
//    fmt.Printf("    taskmap: %v\n", taskmap)
//
//    err := yaml.Unmarshal([]byte(data), &configs)
//    if err != nil {
//            log.Fatalf("error: %v", err)
//    }
//
//    fmt.Printf("--- t:\n%v\n\n", configs)
//
//
//    task := configs["task4"]
//    marshalled, err := yaml.Marshal(&task)
//    println("marshalled task is ", string(marshalled))
//    //configs["task4"].name = "setting afterwards"
//
//    println("descr is ", configs["task4"].descr)
//    println(configs["task4"].cmd)


}
 */
