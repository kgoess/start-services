package main

import (
        "flag"
        "fmt"
        "log"
        "gopkg.in/yaml.v2"
        "os"
        "io/ioutil"
)

type TaskConfig struct {
    Name string
    After []string
    Cmd []string
    Descr string
}


var taskYamlPath = flag.String("taskfile", "", "path to yaml with tasks")
var showConfigsFlag = flag.Bool("showconfigs", false, "dump the configs")

func main(){
    flag.Parse()
	if len(*taskYamlPath) == 0 {
		flag.Usage()
		os.Exit(1)
	}
    configs, afters := loadConfigs(*taskYamlPath)

    if *showConfigsFlag {
        showConfigs(configs, afters)
        return
    }

    for _, task := range configs {
        runTask(task);
    }
}

func runTask(task TaskConfig){

    fmt.Printf("running task %v\n", task.Cmd);
}



func loadConfigs(taskYamlPath string ) (configs map[string]TaskConfig, afters map[string][]TaskConfig) {

    yamlBytes, err := ioutil.ReadFile(taskYamlPath)
    if err != nil {
        log.Fatalf("error: %v", err)
    }

    configs = map[string]TaskConfig{}

    err = yaml.Unmarshal(yamlBytes, &configs)
    if err != nil {
        log.Fatalf("error: %v", err)
    }

    afters = map[string][]TaskConfig{}

    for taskName, task := range configs {
        // TODO this apparently is only a copy? how to get a pointer?
        task.Name = taskName
        slice := make([]TaskConfig, 5, 5) // lenght of x with room for y more

        // TODO make range dynamic, set to correct one
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
