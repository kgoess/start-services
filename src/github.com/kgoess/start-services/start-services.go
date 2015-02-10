package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

type TaskConfig struct {
	Name         string
	After        []string
	Cmd          []string
	Descr        string
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

var taskYamlPath = flag.String("taskfile", "", "path to yaml with tasks")
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

	//fmt.Printf("In task '%s' waiting for %v channels\n", task.Name, task.NumToWaitFor);
	for i := 0; i < task.NumToWaitFor; i++ {
		select {
		case <-task.WaitFor:
		}
	}

	runTask(task, taskResultsChan)

	//fmt.Printf("Done with task '%s' now telling tasks %v they can proceed\n",
	//        task.Name, task.WhenDoneTell)
	for _, ch := range task.WhenDoneTell {
		ch <- true
	}
}

func runTask(task TaskConfig, taskResultsChan chan<- taskResultsMsg) {

	t0 := time.Now()
    app := task.Cmd[0]
	args := task.Cmd[1:]

	fmt.Printf("--------------Running task '%s' %v\n", task.Name, task.Cmd)

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
	fmt.Fprintf(os.Stdout,
		"--------------Finished '%v' success: %5v [%v ms]----------------\n%v\n\n",
		msg.name, msg.succeeded, msg.duration, msg.msg,
	)
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
	return configs, afters
}

func showConfigs(configs map[string]TaskConfig, afters map[string][]TaskConfig) {
	for taskName, task := range configs {
		fmt.Printf("task:  %v (run after: %v)\ndescr: %v\n\t%v\n\tNumToWaitFor: %v\n\tWhenDoneTell: %v\n",
			taskName, task.After, task.Descr, task.Cmd, task.NumToWaitFor, task.WhenDoneTell)
		afterList, exists := afters[taskName]
		if exists {
			for _, after := range afterList {
				if after.Name != "" {
					fmt.Printf("after this we'll run: %s\n", after.Name)
				}
			}
		}
		fmt.Printf("------------------\n")
	}
}
