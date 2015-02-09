package main

import (
        "flag"
        "fmt"
        "gopkg.in/yaml.v2"
        "io/ioutil"
        "log"
        "os"
        "sync"
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

    // give each task its own channel to report 'done' on
    taskDoneChs := make(map[string]chan bool)
    for taskName, _ := range configs {
        taskDoneChs[taskName] = make(chan bool)
    }

    waitForChsForTask := make(map[string] []chan bool)

    // set up the do-after parts to wait for
    for taskName, task := range configs {
        waitForChs := make([]chan bool, 0, 5)
        for _, afterKey := range task.After {
            //see http://blog.golang.org/go-slices-usage-and-internals
            //    var p []int // == nil

            fmt.Printf("after %s we'll run %s\n", afterKey, taskName);

            if len(waitForChs) == cap(waitForChs) {
                newSlice := make([]chan bool, len(waitForChs) +1, len(waitForChs) + 5)
                waitForChs = newSlice
            }
            waitForChs = append(waitForChs, taskDoneChs[afterKey])
            fmt.Printf("%v\n", waitForChs)
            waitForChsForTask[taskName] = waitForChs
        }
    }

    // now actually run them
    var wg sync.WaitGroup

    for taskName, task := range configs {
        whenFinishedCallCh := taskDoneChs[taskName]
        waitForChs := waitForChsForTask[taskName]
        wg.Add(1)
        go runTask(task, waitForChs, whenFinishedCallCh, &wg);
    }
    wg.Wait()

}

func runTask(
        task *TaskConfig,
        waitForChs []chan bool,
        whenFinishedCallCh chan bool,
        wg *sync.WaitGroup,
    ){

    fmt.Printf("running task %s %v, after %s will respond on chan %v\n",
            task.Name, task.Cmd, waitForChs, whenFinishedCallCh);
    wg.Done()
}



func loadConfigs(taskYamlPath string ) (configsPtrs map[string]*TaskConfig, afters map[string][]TaskConfig) {

    yamlBytes, err := ioutil.ReadFile(taskYamlPath)
    if err != nil {
        log.Fatalf("error: %v", err)
    }

    configs := map[string]TaskConfig{}

    err = yaml.Unmarshal(yamlBytes, &configs)
    if err != nil {
        log.Fatalf("error: %v", err)
    }

    configsPtrs = map[string]*TaskConfig{}
    afters = map[string][]TaskConfig{}

    for taskName, _ := range configs {
        // "range configs" just gives us the values for each key, we want a pointer 
        // to the original so we can modify it
        task := configs["taskName"]
        task.Name = taskName
        configsPtrs[taskName] = &task
        slice := make([]TaskConfig, 0, 5) // length of x with room for y more

        for _, after := range task.After {
            if len(slice) == cap(slice) {
                newSlice := make([]TaskConfig, len(slice)+1, len(slice)+5)
                copy(newSlice, slice)
            }
            afters[after] = slice
            afters[after] = append(afters[after], task)
        }
    }

    return configsPtrs, afters
}
func showConfigs(configs map[string]*TaskConfig, afters map[string][]TaskConfig){
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
