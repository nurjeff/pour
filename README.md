# Logging Package

This package can be used to enhance default logs.

---



### Setup

To use it, import it and then call:

```go
pour.Setup(inDocker)
```


When first starting the program, a file `config_pour.json` will automatically be created with the content:


```json
{
	"remote_logs": true, 
	"project_key": "<ASK ADMINISTRATOR FOR KEY>", 
	"host": "127.0.0.1", 
	"port": 12555, 
	"client": "default_user", 
	"client_key": "b930ffce-d388-43fc-aa1a-13962a7d6bc9" 
}
```

You can leave most of it as-is, just replace the `project_key` with one received from any administrator.

Logs will be synced to a remote logging server, as well as saved in an automatically created `logs` folder.

---

Available methods:

`Log()` -> Just write a default log, analogue to `log.Println()`

`LogError(err)` -> Write a default error log, this will be marked in the GUI

`LogPanicKill(1, err)` -> Logs a panic and also exits the program with a panic stack

`LogTagged(false, pour.TAG_SUCCESS, "Success!")` -> Logs a success message, this will be marked in the GUI

Available Tags are:

* TAG_SUCCESS
* TAG_
