// +build !redskull
package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/sentinel-tools/sconf-parser"
	"github.com/therealbill/libredis/client"
)

var sentinel parser.SentinelConfig

//getPod(podname) returns eitehr an empty Pod struct and error, or a populated
//PodConfig for the podname given
func getPod(podname string) (pod parser.PodConfig, err error) {
	pod, err = sentinel.GetPod(podname)
	if err != nil {
		log.Print(err)
	}
	return pod, err
}

func Reset(pod *parser.PodConfig) error {
	// loop over list of sentinels, issue a reset
	sentinels, err := pod.GetSentinels()
	if err != nil {
		log.Print(err.Error())
	}
	resets := 0
	for _, s := range sentinels {
		sc, err := client.DialAddress(s)
		if err != nil {
			log.Print(err.Error())
			continue
		}
		err = sc.SentinelReset(pod.Name)
		if err != nil {
			log.Print(err.Error())
			continue
		}
		resets++
	}
	if resets != len(sentinels) {
		return fmt.Errorf("Only %d of %d sentinels were successfully reset", resets, len(sentinels))
	}
	return nil
}

// LiveSlaves() returns a list of connections to slaves. it can be empty if no
// slaves exist or no slaves are reachable
func LiveSlaves(pod parser.PodConfig) []*client.Redis {
	slaves := pod.KnownSlaves
	var live []*client.Redis
	for _, s := range slaves {
		sc, err := client.DialWithConfig(&client.DialConfig{Address: s, Password: pod.Authpass})
		if err != nil {
			log.Print(err.Error())
			continue
		}
		live = append(live, sc)
	}
	return live
}

// CheckAuth() will attempt to connect to the master and validate we can auth
// by issuing a ping
func CheckAuth(pod *parser.PodConfig) (map[string]bool, error) {
	addr := fmt.Sprintf("%s:%s", pod.MasterIP, pod.MasterPort)
	results := make(map[string]bool)
	invalid := false
	dc := client.DialConfig{Address: addr, Password: pod.Authpass}
	c, err := client.DialWithConfig(&dc)
	if err != nil {
		if !strings.Contains(err.Error(), "invalid password") {
			log.Print("Unable to connect to %s. Error: %s", addr, err.Error())
		}
		results["master"] = false
	} else {
		err = c.Ping()
		if err != nil {
			log.Print(err)
			results["master"] = false
			invalid = true
		} else {
			results["master"] = true
		}
	}

	for _, slave := range LiveSlaves(*pod) {
		sid := fmt.Sprintf(slave.Address())
		if slave.Ping() != nil {
			results[sid] = false
			invalid = true
			continue
		} else {
			results[sid] = true
		}
	}
	if invalid {
		err = errors.New("At least one node in pod failed auth check")
	}
	return results, err
}

// ValidateSentinels() iterates over KnownSentinels, connecting to each This is
// useufl for confirming the number of known sentinels matches the number of
// sentinels available
func ValidateSentinels(pod *parser.PodConfig) (bool, error) {
	sentinels, err := pod.GetSentinels()
	if err != nil {
		return false, err
	}
	failed := 0
	connected := 0
	for _, s := range sentinels {
		sc, err := client.DialAddress(s)
		if err != nil {
			log.Print(s, err.Error())
			failed++
			continue
		}
		master, err := sc.SentinelMaster(pod.Name)
		if err != nil {
			log.Printf("[%s] %s", s, err.Error())
			failed++
			continue
		}
		if master.Name != pod.Name {
			log.Printf("Wierd, request master for pod '%s', got master for pod '%s'", pod.Name, master.Name)
			failed++
			continue
		} else {
			connected++
		}
	}
	if len(sentinels) > connected {
		return false, fmt.Errorf("%d of %d sentinels were contacted and has this pod in their list", connected, len(sentinels))
	}
	return true, nil
}
