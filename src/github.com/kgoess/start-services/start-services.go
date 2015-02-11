package main

import (
	"flag"
	"fmt"
	ansi "github.com/mgutz/ansi"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

type TaskConfig struct {
	// these are set in the yaml file
	Name  string
	After []string
	Cmd   []string
	Descr string
	// these are set up in loadConfigs
	NumToWaitFor int
	WaitFor      chan bool
	WhenDoneTell []chan bool
}

type taskResultsMsg struct {
	name      string
	succeeded bool
	msg       string
	duration  int64
}

var taskYamlPath = flag.String("taskfile", "", "[required] path to yaml with tasks")
var showConfigsFlag = flag.Bool("showconfigs", false, "dump the configs")

func main() {
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

	t0 := time.Now()

	// now actually run them
	taskResultsChan := make(chan taskResultsMsg)

	for _, task := range configs {
		go handleTask(task, taskResultsChan)
	}

	// and show the results
	showOutput(len(configs), taskResultsChan)

	t1 := time.Now()
	duration := int64(t1.Sub(t0) / time.Millisecond)
	showFinalMsg(len(configs), duration)
}

func handleTask(
	task TaskConfig,
	taskResultsChan chan<- taskResultsMsg,
) {

	someFailed := false

	//fmt.Printf("In task '%s' waiting for %v channels\n", task.Name, task.NumToWaitFor);
	for i := 0; i < task.NumToWaitFor; i++ {
		select {
		case result := <-task.WaitFor:
			if !result {
				someFailed = true
			}
		}
	}

	var taskRanOk bool
	if someFailed {
		fmt.Printf(
			ansi.Color("--------------Not running '%s' because of failures in the chain\n", "red+bh"),
			task.Name)
		taskRanOk = false
		msg := taskResultsMsg{
			name:      task.Name,
			succeeded: false,
			msg:       "Didn't run because of previous job failures",
			duration:  0,
		}
		taskResultsChan <- msg
	} else {
		taskRanOk = runTask(task, taskResultsChan)
	}

	//fmt.Printf("Done with task '%s' now telling tasks %v they can proceed\n",
	//        task.Name, task.WhenDoneTell)
	payItForward := true
	if someFailed || !taskRanOk {
		payItForward = false
	}
	for _, ch := range task.WhenDoneTell {
		ch <- payItForward
	}
}

func runTask(task TaskConfig, taskResultsChan chan<- taskResultsMsg) bool {

	t0 := time.Now()
	app := task.Cmd[0]
	args := task.Cmd[1:]

	fmt.Printf(
		ansi.Color("--------------Running task '%s' %v\n", "cyan"),
		task.Name, task.Cmd,
	)

	cmd := exec.Command(app, args...)

	stdout, err := cmd.CombinedOutput()
	stdoutStr := string(stdout[:])
	if err != nil {
		stdoutStr += err.Error()
	}
	t1 := time.Now()

	duration := int64(t1.Sub(t0) / time.Millisecond)

	msg := taskResultsMsg{
		name:      task.Name,
		succeeded: err == nil,
		msg:       stdoutStr,
		duration:  duration,
	}

	taskResultsChan <- msg

	return err == nil
}

func showOutput(numTasks int, taskResultsChan <-chan taskResultsMsg) {

	for i := 0; i < numTasks; i++ {
		select {
		case msg := <-taskResultsChan:
			printTaskResults(msg)
		}
	}
}

func showFinalMsg(numJobs int, duration int64) {

	fmt.Printf("Finished %d tasks in %d ms\n\n", numJobs, duration)

}

func printTaskResults(msg taskResultsMsg) {

	var successColor string
	if msg.succeeded == true {
		successColor = "green+bh"
	} else {
		successColor = "red+bh"
	}

	outstr := fmt.Sprintf(
		ansi.Color("--------------Finished '%v' ", "green+bh")+
			ansi.Color("success: %5v ", successColor)+
			ansi.Color("[%v ms]----------------\n", "green+bh")+
			ansi.Color("%v\n\n", "cyan"),
		msg.name, msg.succeeded, msg.duration, msg.msg,
	)

	fmt.Fprintf(os.Stdout, outstr)
}

func loadConfigs(taskYamlPath string) (configs map[string]TaskConfig, afters map[string][]TaskConfig) {

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

	// set task.Name, and collect afters in a list
	for taskName, _ := range configs {
		// "range configs" just gives us the values for each key, we want a pointer
		// to the original so we can modify it
		task := configs[taskName]
		task.Name = taskName

		for _, after := range task.After {
			afters[after] = append(afters[after], task)
		}
		// replace the original with our altered copy
		configs[taskName] = task
	}

	// look at all the things that were named in an "after", and set up
	// a channel to they need to call when they're done
	for thisTaskName, afters := range afters {
		thisTask := configs[thisTaskName]
		for _, callWhenFinished := range afters {
			callWhenFinished := configs[callWhenFinished.Name]
			if callWhenFinished.WaitFor == nil {
				callWhenFinished.WaitFor = make(chan bool, 5)
			}
			aChan := callWhenFinished.WaitFor
			thisTask.WhenDoneTell = append(thisTask.WhenDoneTell, aChan)
			callWhenFinished.NumToWaitFor++

			// since we're dealing with copies, need to put the new values back
			// into the list
			configs[thisTaskName] = thisTask
			configs[callWhenFinished.Name] = callWhenFinished
		}
	}

    for _, task := range configs {
        seen := make(map[string] bool)
        seen[ task.Name ] = true
        checkNoCircular(task, afters, seen, configs)
    }
	return configs, afters
}

// We look at this task and follow the list of tasks scheduled to come after it.
// If we see any task twice, it's circular
func checkNoCircular(
        task TaskConfig,
        afters map[string][]TaskConfig,
        seen map[string] bool,
        configs map[string]TaskConfig,
    ){
    for _, nextTask := range afters[task.Name] {
        if seen[nextTask.Name] {
            showConfigs(configs, afters)
            fmt.Fprintf(os.Stderr,
                    ansi.Color("circular dependency in %s!\n", "red+bh"),
                    nextTask.Name,
                )
            os.Exit(1)
        }
        seen[nextTask.Name] = true
        copySeen := make(map[string] bool)
        for k, v := range seen {
            copySeen[k] = v
        }
        checkNoCircular(nextTask, afters, copySeen, configs)
    }
}

func showConfigs(configs map[string]TaskConfig, afters map[string][]TaskConfig) {
	for taskName, task := range configs {
		fmt.Printf("task:  %v (run after: %v)\ndescr: %v\n\t%v\n",
			taskName, task.After, task.Descr, task.Cmd)
		afterList, exists := afters[taskName]
		if exists {
			for _, after := range afterList {
				if after.Name != "" {
					fmt.Printf("after this we can run: %s\n", after.Name)
				}
			}
		}
		fmt.Printf("------------------\n")
	}
}
