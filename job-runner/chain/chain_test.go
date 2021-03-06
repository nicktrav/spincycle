// Copyright 2017-2018, Square, Inc.

package chain

import (
	"reflect"
	"sort"
	"testing"

	"github.com/square/spincycle/proto"
	testutil "github.com/square/spincycle/test"
)

func TestNewChain(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: map[string]proto.Job{
			"job1": proto.Job{
				Id:    "job1",
				State: proto.STATE_COMPLETE,
			},
			"job2": proto.Job{
				Id:    "job2",
				State: proto.STATE_FAIL,
			},
			"job3": proto.Job{
				Id:    "job3",
				State: proto.STATE_STOPPED,
			},
			"job4": proto.Job{
				Id:    "job4",
				State: proto.STATE_UNKNOWN,
			},
			"job5": proto.Job{
				Id:    "job5",
				State: proto.STATE_RUNNING,
			},
			"job6": proto.Job{
				Id:    "job6",
				State: proto.STATE_PENDING,
			},
		},
	}

	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedJobStates := map[string]byte{
		"job1": proto.STATE_COMPLETE,
		"job2": proto.STATE_FAIL,
		"job3": proto.STATE_STOPPED,
		"job4": proto.STATE_FAIL,
		"job5": proto.STATE_FAIL,
		"job6": proto.STATE_PENDING,
	}
	for jobId, expectedState := range expectedJobStates {
		if c.JobState(jobId) != expectedState {
			t.Errorf("%s state = %d, expected state %d", jobId, c.JobState(
				jobId), expectedState)
		}
	}

	// Test NumJobsRun
	c.SetJobState("job6", proto.STATE_RUNNING)
	running := c.Running()
	if running["job6"].N != 5 { // (jobs 1, 2, 4, 5, + 6) -> NumJobsRun = 5
		t.Errorf("got NumJobsRun = %d, expected %d", running["job6"].N, 5)
	}
}

func TestFirstJobMultiple(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job3"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	_, err := c.FirstJob()
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestFirstJobOne(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedFirstJob := jc.Jobs["job1"]
	firstJob, err := c.FirstJob()

	if !reflect.DeepEqual(firstJob, expectedFirstJob) {
		t.Errorf("firstJob = %v, expected %v", firstJob, expectedFirstJob)
	}
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
}

func TestLastJobMultiple(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	_, err := c.lastJob()
	if err == nil {
		t.Errorf("expected an error, but did not get one")
	}
}

func TestLastJobOne(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedLastJob := jc.Jobs["job4"]
	lastJob, err := c.lastJob()

	if !reflect.DeepEqual(lastJob, expectedLastJob) {
		t.Errorf("lastJob = %v, expected %v", lastJob, expectedLastJob)
	}
	if err != nil {
		t.Errorf("err = %s, expected nil", err)
	}
}

func TestRunnableJobs(t *testing.T) {
	// Job chain:
	//       2 - 5
	//      / \
	// -> 1    4
	//     \  /
	//      3
	// Job 3 and 5 should be runnable

	jobs := map[string]proto.Job{
		"job1": proto.Job{
			Id:            "job1",
			State:         proto.STATE_COMPLETE,
			SequenceId:    "job1",
			SequenceRetry: 1,
		},
		"job2": proto.Job{
			Id:         "job2",
			State:      proto.STATE_COMPLETE,
			SequenceId: "job1",
		},
		"job3": proto.Job{ // can be run
			Id:         "job3",
			State:      proto.STATE_STOPPED,
			SequenceId: "job1",
			Retry:      1,
		},
		"job4": proto.Job{
			Id:         "job4",
			State:      proto.STATE_PENDING,
			SequenceId: "job1",
		},
		"job5": proto.Job{ // can be run
			Id:         "job5",
			State:      proto.STATE_PENDING,
			SequenceId: "job1",
		},
	}
	jc := &proto.JobChain{
		RequestId: "resume",
		Jobs:      jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job4"},
		},
	}
	sjc := &proto.SuspendedJobChain{
		RequestId: "resume",
		JobChain:  jc,
		TotalJobTries: map[string]uint{
			"job1": 2, // sequence retried once
			"job2": 2,
			"job3": 3,
			"job4": 1,
		},
		LatestRunJobTries: map[string]uint{
			"job1": 1,
			"job2": 1,
			"job3": 2, // job3 should have 1 try left
			"job4": 1,
		},
		SequenceTries: map[string]uint{
			"job1": 1,
		},
	}
	c := NewChain(sjc.JobChain, sjc.SequenceTries, sjc.TotalJobTries, sjc.LatestRunJobTries)

	expectedJobs := proto.Jobs{jc.Jobs["job3"], jc.Jobs["job5"]}
	sort.Sort(expectedJobs)
	runnableJobs := c.RunnableJobs()
	sort.Sort(runnableJobs)

	if !reflect.DeepEqual(runnableJobs, expectedJobs) {
		t.Errorf("runnableJobs = %v, want %v", runnableJobs, expectedJobs)
	}
}

func TestNextJobs(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedNextJobs := proto.Jobs{jc.Jobs["job2"], jc.Jobs["job3"]}
	sort.Sort(expectedNextJobs)
	nextJobs := c.NextJobs("job1")
	sort.Sort(nextJobs)

	if !reflect.DeepEqual(nextJobs, expectedNextJobs) {
		t.Errorf("nextJobs = %v, want %v", nextJobs, expectedNextJobs)
	}

	nextJobs = c.NextJobs("job4")

	if len(nextJobs) != 0 {
		t.Errorf("nextJobs count = %d, want 0", len(nextJobs))
	}
}

func TestPreviousJobs(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedPreviousJobs := proto.Jobs{jc.Jobs["job2"], jc.Jobs["job3"]}
	sort.Sort(expectedPreviousJobs)
	previousJobs := c.previousJobs("job4")
	sort.Sort(previousJobs)

	if !reflect.DeepEqual(previousJobs, expectedPreviousJobs) {
		t.Errorf("previousJobs = %v, want %v", previousJobs, expectedPreviousJobs)
	}

	previousJobs = c.previousJobs("job1")

	if len(previousJobs) != 0 {
		t.Errorf("previousJobs count = %d, want 0", len(previousJobs))
	}
}

func TestIsRunnable(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(6),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3", "job5"},
			"job2": {"job4", "job6"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_PENDING)
	c.SetJobState("job6", proto.STATE_STOPPED)
	c.SetLatestRunJobTries("job6", 1) // tried once before stop

	// Job 1 has already been run
	expectedRunnable := false
	runnable := c.IsRunnable("job1")

	if runnable != expectedRunnable {
		t.Errorf("runnable = %t, want %t", runnable, expectedRunnable)
	}

	// Job 4 can't run until job 3 is complete
	expectedRunnable = false
	runnable = c.IsRunnable("job4")

	if runnable != expectedRunnable {
		t.Errorf("runnable = %t, want %t", runnable, expectedRunnable)
	}

	// Job 5 can run (because Job 1 is done)
	expectedRunnable = true
	runnable = c.IsRunnable("job5")

	if runnable != expectedRunnable {
		t.Errorf("runnable = %t, want %t", runnable, expectedRunnable)
	}

	// Job 6 can run (stopped but can be retried)
	expectedRunnable = true
	runnable = c.IsRunnable("job6")

	if runnable != expectedRunnable {
		t.Errorf("runnable = %t, want %t", runnable, expectedRunnable)
	}
}

// The chain is not done or complete - a job is running.
func TestIsDoneJobRunning(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_RUNNING)

	expectedDone := false
	expectedComplete := false
	done, complete := c.IsDoneRunning()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

// The chain is not done or complete - more jobs can be run.
func TestIsDoneJobCanBeRun(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_PENDING)
	// ^ Job 4 can still be run

	expectedDone := false
	expectedComplete := false
	done, complete := c.IsDoneRunning()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

// The chain is not done or complete - more jobs can be run.
func TestIsDoneJobRetry(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_FAIL)
	// ^ Job 4 can still be run

	expectedDone := false
	expectedComplete := false
	done, complete := c.IsDoneRunning()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

// When the chain is done but not complete.
func TestIsDoneNotComplete(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_FAIL)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_PENDING)

	expectedDone := true
	expectedComplete := false
	done, complete := c.IsDoneRunning()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}

	// Make sure we can handle unknown states
	c.SetJobState("job4", proto.STATE_UNKNOWN)

	done, complete = c.IsDoneRunning()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

// When the chain is done and complete.
func TestIsDoneComplete(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_COMPLETE)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)

	expectedDone := true
	expectedComplete := true
	done, complete := c.IsDoneRunning()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

// When the chain is not done or complete because stopped jobs are treated like
// pending jobs.
func TestIsDoneJobStopped(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_STOPPED)
	c.SetJobState("job3", proto.STATE_COMPLETE)
	c.SetJobState("job4", proto.STATE_COMPLETE)

	expectedDone := false
	expectedComplete := false
	done, complete := c.IsDoneRunning()

	if done != expectedDone || complete != expectedComplete {
		t.Errorf("done = %t, complete = %t, want %t and %t", done, complete, expectedDone, expectedComplete)
	}
}

func TestSetJobState(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(1),
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	c.SetJobState("job1", proto.STATE_COMPLETE)
	if jc.Jobs["job1"].State != proto.STATE_COMPLETE {
		t.Errorf("State = %d, want %d", jc.Jobs["job1"].State, proto.STATE_COMPLETE)
	}
}

func TestSetState(t *testing.T) {
	jc := &proto.JobChain{}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	c.SetState(proto.STATE_RUNNING)
	if c.State() != proto.STATE_RUNNING {
		t.Errorf("State = %d, want %d", c.State(), proto.STATE_RUNNING)
	}
}

func TestIndegreeCounts(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(9),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5"},
			"job3": {"job5", "job6"},
			"job4": {"job6", "job7"},
			"job5": {"job6"},
			"job6": {"job8"},
			"job7": {"job8"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedCounts := map[string]int{
		"job1": 0,
		"job2": 1,
		"job3": 1,
		"job4": 1,
		"job5": 2,
		"job6": 3,
		"job7": 1,
		"job8": 2,
		"job9": 0,
	}
	counts := c.indegreeCounts()

	if !reflect.DeepEqual(counts, expectedCounts) {
		t.Errorf("counts = %v, want %v", counts, expectedCounts)
	}
}

func TestOutdegreeCounts(t *testing.T) {
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(7),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4", "job5", "job6"},
			"job3": {"job5", "job6"},
			"job4": {"job5", "job6"},
			"job5": {"job6"},
			"job6": {"job7"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedCounts := map[string]int{
		"job1": 2,
		"job2": 3,
		"job3": 2,
		"job4": 2,
		"job5": 1,
		"job6": 1,
		"job7": 0,
	}
	counts := c.outdegreeCounts()

	if !reflect.DeepEqual(counts, expectedCounts) {
		t.Errorf("counts = %v, want %v", counts, expectedCounts)
	}
}

func TestIsAcyclic(t *testing.T) {
	// No cycle in the chain.
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedIsAcyclic := true
	isAcyclic := c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}

	// Cycle from end to beginning of the chain (i.e., there is no first job).
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job4"},
			"job4": {"job1"},
		},
	}
	c = NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedIsAcyclic = false
	isAcyclic = c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}

	// Cycle in the middle of the chain.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(4),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
			"job2": {"job4"},
			"job3": {"job5"},
			"job4": {"job5"},
			"job5": {"job2", "job6"},
		},
	}
	c = NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedIsAcyclic = false
	isAcyclic = c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}

	// No cycle, but multiple first jobs and last jobs.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(5),
		AdjacencyList: map[string][]string{
			"job1": {"job3"},
			"job2": {"job3"},
			"job3": {"job4", "job5"},
		},
	}
	c = NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedIsAcyclic = true
	isAcyclic = c.isAcyclic()

	if isAcyclic != expectedIsAcyclic {
		t.Errorf("isAcyclic = %t, want %t", isAcyclic, expectedIsAcyclic)
	}
}

func TestValidateAdjacencyList(t *testing.T) {
	// Invalid 1.
	jc := &proto.JobChain{
		Jobs: testutil.InitJobs(2),
		AdjacencyList: map[string][]string{
			"job1": {"job2", "job3"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedValid := false
	valid := c.adjacencyListIsValid()

	if valid != expectedValid {
		t.Errorf("valid = %t, expected %t", valid, expectedValid)
	}

	// Invalid 2.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(2),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job7": {},
		},
	}
	c = NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedValid = false
	valid = c.adjacencyListIsValid()

	if valid != expectedValid {
		t.Errorf("valid = %t, expected %t", valid, expectedValid)
	}

	// Valid.
	jc = &proto.JobChain{
		Jobs: testutil.InitJobs(3),
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
		},
	}
	c = NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expectedValid = true
	valid = c.adjacencyListIsValid()

	if valid != expectedValid {
		t.Errorf("valid = %t, expected %t", valid, expectedValid)
	}
}

func TestSequenceStartJob(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expect := jobs["job1"]
	actual := c.SequenceStartJob("job2")

	if !reflect.DeepEqual(actual, expect) {
		t.Errorf("sequence start job= %v, expected %v", actual, expect)
	}
}

func TestIsSequenceStartJobs(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	if c.IsSequenceStartJob("job2") {
		t.Errorf("got true that job2 is a sequence start job, expected false")
	}
	if !c.IsSequenceStartJob("job1") {
		t.Errorf("got that job1 is not a sequence start job, expected true")
	}
}

func TestCanRetrySequenceTrue(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	expect := true
	actual := c.CanRetrySequence("job2")

	if actual != expect {
		t.Errorf("can retry sequence = %v, expected %v", actual, expect)
	}
}

func TestCanRetrySequenceFalse(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	// 2 retries are configured for the sequence job2 is in
	jobId := "job2"
	// Increment sequence tries thrice to exhaust retries
	c.IncrementSequenceTries(jobId)
	c.IncrementSequenceTries(jobId)
	c.IncrementSequenceTries(jobId)

	expect := false
	actual := c.CanRetrySequence(jobId)

	if actual != expect {
		t.Errorf("can retry sequence = %v, expected %v", actual, expect)
	}
}

func TestIncrementSequenceTries(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	jobId := "job2"
	c.IncrementSequenceTries(jobId)

	expect := uint(1)
	actual := c.SequenceTries(jobId)

	if actual != expect {
		t.Errorf("sequence tries= %v, expected %v", actual, expect)
	}
}

func TestSequenceTries(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))

	jobId := "job2"

	expect := uint(0)
	actual := c.SequenceTries(jobId)

	if actual != expect {
		t.Errorf("sequence tries= %v, expected %v", actual, expect)
	}
}

func TestIsDoneRetryableSequenceFalse(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_FAIL)

	expectDone := false
	expectComplete := false
	actualDone, actualComplete := c.IsDoneRunning()

	if actualDone != expectDone || actualComplete != expectComplete {
		t.Errorf("done = %v, expected %v. complete = %v, expected %v.", actualDone, expectDone, actualComplete, expectComplete)
	}
}

func TestIsDoneRetryableSequenceTrue(t *testing.T) {
	jobs := testutil.InitJobsWithSequenceRetry(4, 2)
	jc := &proto.JobChain{
		Jobs: jobs,
		AdjacencyList: map[string][]string{
			"job1": {"job2"},
			"job2": {"job3"},
			"job3": {"job4"},
		},
	}
	c := NewChain(jc, make(map[string]uint), make(map[string]uint), make(map[string]uint))
	c.IncrementSequenceTries("job1")
	c.SetJobState("job1", proto.STATE_COMPLETE)
	c.SetJobState("job2", proto.STATE_FAIL)

	// Simulate exhausting sequence retries
	failedJobId := "job2"
	c.IncrementSequenceTries(failedJobId)
	c.IncrementSequenceTries(failedJobId)

	expectDone := true
	expectComplete := false
	actualDone, actualComplete := c.IsDoneRunning()

	if actualDone != expectDone || actualComplete != expectComplete {
		t.Errorf("done = %v, expected %v. complete = %v, expected %v.", actualDone, expectDone, actualComplete, expectComplete)
	}
}
