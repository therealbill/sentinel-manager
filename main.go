package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"text/template"

	"github.com/codegangsta/cli"
	"github.com/kelseyhightower/envconfig"
	"github.com/sentinel-tools/sconf-parser"
	"github.com/therealbill/libredis/client"
	"github.com/therealbill/libredis/structures"
)

type LaunchConfig struct {
	SentinelConfigFile string
}

var (
	config LaunchConfig
	enc    *json.Encoder
	pod    *parser.PodConfig
	app    *cli.App
	errlog *log.Logger
)

func init() {
	// set up error logger
	errlog = log.New(os.Stderr, "[ERROR]", log.Lshortfile|log.LstdFlags)
	// set standard logger to stdout
	log.SetOutput(os.Stdout)
	err := envconfig.Process("sentinel-manager", &config)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	app = cli.NewApp()
	app.Name = "sentinel-manager"
	app.Usage = "Interact with a Sentinel using configuration data"
	app.Version = "0.1"
	app.EnableBashCompletion = true
	author := cli.Author{Name: "Bill Anderson", Email: "therealbill@me.com"}
	app.Authors = append(app.Authors, author)
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "address, a",
			Value: "localhost",
			Usage: "Address of the sentinel to talk to",
		},
		cli.IntFlag{
			Name:  "port, p",
			Value: 26379,
			Usage: "Port of the sentinel to talk to",
		},
		cli.BoolFlag{
			Name:  "walkpod,w",
			Usage: "Walk each pod for additional sentinels to operate on.",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "addpod",
			Usage:  "Add a pod to the specified sentinel",
			Action: AddPod,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "Name of the pod",
				},
				cli.StringFlag{
					Name:  "address, a",
					Usage: "hostname/IP of master",
				},
				cli.StringFlag{
					Name:  "password, t",
					Usage: "The password to use (will be used to set amster-auth as well)",
					Value: "USE-A-REAL-PASSWORD",
				},
				cli.StringFlag{
					Name:  "reconfigure-script,r",
					Usage: "Script to use on failover event",
				},
				cli.StringFlag{
					Name:  "notification-script,o",
					Usage: "Script to use on failover event",
				},
				cli.IntFlag{
					Name:  "port, p",
					Usage: "Port the master is running on",
					Value: 6379,
				},
				cli.IntFlag{
					Name:  "quorum, q",
					Usage: "Quorum for event triggers",
					Value: 2,
				},
			},
		},
		{
			Name:   "set",
			Usage:  "Set a directive's value on *every* pod in Sentinel",
			Action: SetOnAllPods,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "directive, d",
					Usage: "Directive to set",
				},
				cli.StringFlag{
					Name:  "value, v",
					Usage: "Value to set",
				},
			},
		},
		{
			Name:   "reset",
			Usage:  "Reset every pod in Sentinel",
			Action: resetAllPods,
		},
		{
			Name:   "removepod",
			Usage:  "Remove pod from the specified sentinel",
			Action: RemovePod,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "Name of the pod",
				},
				cli.StringFlag{
					Name:  "archive, a",
					Usage: "Archive configuration locally before deleting",
				},
			},
		},
	}
	app.Run(os.Args)
}

func getSentinel(c *cli.Context) (sentinel *client.Redis) {
	saddr := fmt.Sprintf("%s:%d", c.GlobalString("address"), c.GlobalInt("port"))
	sentinel, err := client.DialAddress(saddr)
	bailOnError(err)
	return
}

func AddPod(c *cli.Context) {
	name := c.String("name")
	password := c.String("password")
	onreconfig := c.String("reconfigure-script")
	notification := c.String("notification-script")
	host := c.String("address")
	quorum := c.Int("quorum")
	port := c.Int("port")
	sentinel := getSentinel(c)
	ok, err := sentinel.SentinelMonitor(name, host, port, quorum)
	if !ok {
		errlog.Fatalf("on AddPod: err='%v'", err)
	}
	err = sentinel.SentinelSetString(name, "auth-pass", password)
	if err != nil {
		errlog.Fatalf("Error on setting auth-pass: '%v'", err)
	}
	if onreconfig > "" {
		err = sentinel.SentinelSetString(name, "client-reconfig-script", onreconfig)
		if err != nil {
			errlog.Fatalf("Error on setting client-reconfig-script '%v'", err)
		}
	}
	if notification > "" {
		err = sentinel.SentinelSetString(name, "notification-script", notification)
		if err != nil {
			errlog.Fatalf("Error on setting notification-script '%v'", err)
		}
	}

}
func resetAllPods(c *cli.Context) {
	log.Printf("TBI")
}

func SetOnAllPods(c *cli.Context) {
	pods, err := getPodList(c)
	bailOnError(err)
	directive := c.Args()[0]
	value := c.Args()[1]
	sentinel := getSentinel(c)
	log.Printf("Setting %s=%s on all pods...", directive, value)
	for _, pod := range pods {
		log.Printf("Operating on %s", pod.Name)
		err := sentinel.SentinelSetString(pod.Name, directive, value)
		if err != nil {
			errlog.Printf("Unable to set %s to %s on pod '%s'. Error: '%v", directive, value, pod.Name, err)
		}
	}
}

func SetSentinelPod(c *cli.Context) {
	sentinel_directive := c.String("directive")
	value := c.String("value")
	sentinels, err := pod.GetSentinels()
	bailOnError(err)
	for _, s := range sentinels {
		//log.Printf("Updating Sentinel %s", s)
		sentinel, err := client.DialAddress(s)
		if err != nil {
			log.Printf("Unable to connect to %s! You will need to manually adjust the directive's value for this sentinel.", s)
			continue
		}
		sentinel.SentinelSetString(pod.Name, sentinel_directive, value)
	}
}

func RemovePod(c *cli.Context) {
	sentinel := getSentinel(c)
	pod, err := sentinel.SentinelMaster(c.String("name"))
	if c.Bool("archive") {
		podname := c.String("name")
		t := template.Must(template.New("podinfo").Parse(PodInfoTemplate))
		filename := fmt.Sprintf("archive-%s.txt", podname)
		arcfile, err := os.Create(filename)
		if err != nil {
			log.Fatalf("Unable to open %s for writing archive, bailing", filename)
		}
		err = t.Execute(arcfile, pod)
		if err != nil {
			log.Fatal("executing template:", err)
		}
	}
	ok, err := sentinel.SentinelRemove(c.String("name"))
	bailOnError(err)
	if ok {
		log.Printf("Pod %s was removed from sentinel", pod.Name)
	} else {
		log.Printf("Pod not on sentinel")
	}
}

func bailOnError(err error) {
	if err != nil {
		errlog.SetFlags(log.Lshortfile | log.LstdFlags)
		errlog.Output(2, err.Error())
		errlog.SetFlags(log.LstdFlags)
		log.Fatalf("aborting due to error")
	}
}

func getPodList(c *cli.Context) (pods []structures.MasterInfo, err error) {
	sentinel := getSentinel(c)
	pods, err = sentinel.SentinelMasters()
	return
}
