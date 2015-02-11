# start-services
A little learning exercise, how to start services in parallel in go.

Based on a tasks.yaml file like this:

```yaml
    ---
    task1:
      cmd:
        - /bin/echo
        - blah blah blah
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
        - Wheeeee!
      descr: this task goes after 1 and 2
```

You'd get output like this:

   ./bin/start-services  --taskfile data/tasks.yaml

![screenshot](https://cloud.githubusercontent.com/assets/75720/6121168/2cd57928-b093-11e4-8d03-ca1fc3710ba7.png)

If a task fails, any tasks listed in its "after" section will not run.  Any
circular dependencies in the task graph are detected and and exception will
be thrown for them.
