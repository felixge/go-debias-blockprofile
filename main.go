package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/google/pprof/profile"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		rate = flag.Int64("rate", 0, "The rate passed to runtime.SetBlockProfileRate.")
	)
	flag.Parse()

	if *rate == 0 {
		return errors.New("-rate argument is required and must not be 0")
	}

	data, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		return err
	}

	prof, err := profile.Parse(bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	var (
		delayIndex = -1
		countIndex = -1
	)
	for i, st := range prof.SampleType {
		if st.Type == "delay" && st.Unit == "nanoseconds" {
			delayIndex = i
		} else if st.Type == "contentions" && st.Unit == "count" {
			countIndex = i
		} else if st.Type == "unbiased_delay" && st.Unit == "nanoseconds" {
			fmt.Printf("Looks like this file was already debiased. Not changing it again : )\n")
			return nil
		}
	}
	if delayIndex == -1 || countIndex == -1 {
		return fmt.Errorf("Could not find the right sample types. Is this a block profile?")
	}

	prof.SampleType = append(prof.SampleType, &profile.ValueType{
		Type: "unbiased_delay",
		Unit: "nanoseconds",
	})

	debiasCount := 0
	for _, sample := range prof.Sample {
		count := sample.Value[countIndex]
		duration := sample.Value[delayIndex]
		meanDuration := duration / count

		if meanDuration < *rate {
			duration = count * *rate
			debiasCount++
		}
		sample.Value = append(sample.Value, duration)
	}

	if debiasCount == 0 {
		fmt.Printf("Found no biased samples. Your profile should be accurate : )\n")
		return nil
	}

	file, err := os.Create(flag.Arg(0))
	if err != nil {
		return err
	}
	defer file.Close()

	if err := prof.Write(file); err != nil {
		return err
	}
	fmt.Printf("Detected and debiased %d out of %d samples in your file.\n", debiasCount, len(prof.Sample))
	return nil
}
