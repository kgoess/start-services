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
	// these are set up in loadConfigs which links up the jobs to channels
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

	// now actually run them, in parallel
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

	somePreviousFailed := false

	// first wait for any prerequisites to finish and succeed

	//fmt.Printf("In task '%s' waiting for %v channels\n", task.Name, task.NumToWaitFor);
	for i := 0; i < task.NumToWaitFor; i++ {
		select {
		case result := <-task.WaitFor:
			if !result {
				somePreviousFailed = true
			}
		}
	}

	// either handle that failure or run this task

	var msg taskResultsMsg

	if somePreviousFailed {
		fmt.Printf(
			ansi.Color("--------------Not running '%s' because of failures in the chain\n", "red+bh"),
			task.Name)
		msg = taskResultsMsg{
			name:      task.Name,
			succeeded: false,
			msg:       "Didn't run because of previous job failures",
			duration:  0,
		}
	} else {
		msg = runTask(task)
	}

	// tell the output channel what the results were
	taskResultsChan <- msg

	// tell succeeding tasks they can now continue

	//fmt.Printf("Done with task '%s' now telling tasks %v they can proceed\n",
	//        task.Name, task.WhenDoneTell)
	payItForward := msg.succeeded
	for _, ch := range task.WhenDoneTell {
		ch <- payItForward
	}
}

func runTask(task TaskConfig) taskResultsMsg {

	t0 := time.Now()
	app := task.Cmd[0]
	args := task.Cmd[1:]

	fmt.Printf(
		ansi.Color("--------------Running task '%s' %v\n", "cyan"),
		task.Name, task.Cmd,
	)

	cmd := exec.Command(app, args...)

	outputBytes, err := cmd.CombinedOutput()
	outputStr := string(outputBytes[:])
	if err != nil {
		outputStr += err.Error()
	}
	t1 := time.Now()

	duration := int64(t1.Sub(t0) / time.Millisecond)

	msg := taskResultsMsg{
		name:      task.Name,
		succeeded: err == nil,
		msg:       outputStr,
		duration:  duration,
	}

	return msg
}

/* func showOutput
   wait on the results channel and print them to the screen as they come in
*/
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

//The YAML data looks like this:
//
//    task4:
//      after:
//         - task1
//         - task2
//      cmd:
//        - /bin/echo
//        - Wheeeee!
//      descr: this task goes after 1 and 2
//
//which says "run task4 after task1 and task2 have finished successsfully". I
// just thought it read better that way.
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

	// set task.Name, and collect "afters" in a list
	// so we can see what runs after this task, as opposed
	// to what this task runs after
	for taskName, _ := range configs {
		task := configs[taskName]
		task.Name = taskName

		for _, after := range task.After {
			afters[after] = append(afters[after], task)
		}
		// "range configs" gives us copies of the value, so if we alter the
		// data members, we want to replace the original in the map with our
		// altered copy
		configs[taskName] = task
	}

	// look at all the things that were named in an "after", and set up
	// a channel for them to call when they're done, connected to the
	// task that's waiting for them to finish
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
		seen := make(map[string]string)
		seen[task.Name] = "root config"
		checkNoCircular(task, afters, seen, configs)
	}
	return configs, afters
}

// We look at this task and follow the list of tasks scheduled to come after it.
// If we see any task twice, it's circular
func checkNoCircular(
	task TaskConfig,
	afters map[string][]TaskConfig,
	seen map[string]string,
	configs map[string]TaskConfig,
) {
	for _, nextTask := range afters[task.Name] {
		_, alreadySeen := seen[nextTask.Name]
		if alreadySeen {
			showCircularDepsMsg(task.Name, seen, configs, afters)
			os.Exit(1)
		}
		seen[nextTask.Name] = task.Name
		copySeen := make(map[string]string)
		for k, v := range seen {
			copySeen[k] = v
		}
		checkNoCircular(nextTask, afters, copySeen, configs)
	}
}

func showCircularDepsMsg(
	taskName string,
	seen map[string]string,
	configs map[string]TaskConfig,
	afters map[string][]TaskConfig,
) {
	showConfigs(configs, afters)
	outstr := ""
	for k, _ := range seen {
		outstr += k + " -> "
	}
	outstr += taskName
	fmt.Fprintf(os.Stderr,
		ansi.Color("circular dependency for %s from the chain where %v!\n",
			"red+bh"),
		taskName, outstr,
	)
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
