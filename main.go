package main

import (
	"fmt"
	"time"
)

const (
	numReplicas     = 6
	timeoutInterval = 5 * time.Second // 1. detect dead coordinator 2. become coordinator
	syncInterval    = 1 * time.Second
)

type Replica struct {
	id            int
	data          string
	allReplicaChs map[int][]chan string

	isCoordinator ConcurrentBoolean
	isAlive       ConcurrentBoolean
	isInElection  ConcurrentBoolean

	//// channels
	//dataChan           chan string //0
	//electionSignalChan chan string
	//announceChan       chan string //2

	shutdownSignalChan     chan struct{}
	shutdownDataChan       chan struct{}
	shutdownTimeoutChan    chan struct{}
	shutdownSyncChan       chan struct{}
	resetTimeoutSignalChan chan struct{}
}

func (r *Replica) start() {
	fmt.Printf("replica %d: started!\n", r.id)

	r.startListeningSignal()

	if r.isCoordinator.Get() {
		r.startSync()
	} else {
		r.startDataListening()
	}
}

func (r *Replica) startSync() {
	go func() {
		// coordinator thread
		for {
			select {
			case <-r.shutdownSyncChan:
				return
			default:
				fmt.Printf("replica %d: syncing!\n", r.id)
				// push with all other replicas
				for id, cMap := range r.allReplicaChs {
					if id != r.id {
						go func(id int, rid int, rdata string, ch chan string) {
							select {
							case ch <- rdata:
							case <-time.After(timeoutInterval):
								fmt.Printf("Replica %d: unable to send sync data to %d (late callback)\n", rid, id)
							}
						}(id, r.id, r.data, cMap[0])

					}
				}
				time.Sleep(syncInterval)
			}
		}
	}()
}

func (r *Replica) startDataListening() {
	r.startListeningTimeout()
	go func() {
		for {
			select {
			case data := <-r.allReplicaChs[r.id][0]:
				fmt.Printf("replica %d: dataChan: %s\n", r.id, data)
				r.resetTimeoutSignalChan <- struct{}{}
			case <-r.shutdownDataChan:
				return
			}
		}
	}()
}

func (r *Replica) startListeningTimeout() {
	go func() {
		for {
			select {
			case <-r.resetTimeoutSignalChan:
				continue
			case <-time.After(timeoutInterval): // 1. detect dead coordinator
				r.isInElection.mu.Lock()
				if !r.isInElection.Get() {
					fmt.Printf("Replica %d: initiating election due to timeout\n", r.id)
					r.initiateElection()
					r.isInElection.Set(true)
				} else {
					fmt.Printf("Replica %d: isInElection, ignore timeout\n", r.id)
				}
				r.isInElection.mu.Unlock()
			case <-r.shutdownTimeoutChan:
				return
			}
		}
	}()
}

func (r *Replica) startListeningSignal() {
	go func() {
		for {
			select {
			case <-r.allReplicaChs[r.id][1]:
				if r.isCoordinator.Get() {
					fmt.Printf("replica %d: isCoordinator, ignore notify\n", r.id)
				} else {
					r.isInElection.mu.Lock()
					if !r.isInElection.Get() {
						fmt.Printf("replica %d: initiating election as notified\n", r.id)
						r.initiateElection()
						r.isInElection.Set(true)
					} else {
						fmt.Printf("replica %d: isInElection, ignore notify\n", r.id)
					}
					r.isInElection.mu.Unlock()
				}

			case <-r.shutdownSignalChan:
				r.isAlive.Set(false)
				if r.isCoordinator.Get() {
					r.shutdownSyncChan <- struct{}{}
				} else {
					r.shutdownDataChan <- struct{}{}
					r.shutdownTimeoutChan <- struct{}{}
				}
				return
			}
		}
	}()
}

func (r *Replica) initiateElection() {
	go func() {
		// notify higher IDs
		doneChannel := make(chan bool)
		for id, ch := range r.allReplicaChs {
			if id > r.id {
				go func(id int, rid int, ch chan string) {
					fmt.Printf("replica %d: notifying %d - before\n", rid, id)
					select {
					case ch <- fmt.Sprintf("%d", rid):
						doneChannel <- true
					case <-time.After(timeoutInterval):
						fmt.Printf("replica %d: notifying %d - dropped\n", rid, id)
					}
					// it may not be always listening. If no listening, we can't let it not send
				}(id, r.id, ch[1])
			}
		}
		// check answers
		select {
		case <-doneChannel:
			r.isInElection.mu.Lock()
			fmt.Printf("replica %d: higher ID is alive, stopped\n", r.id)
			r.isInElection.Set(false)
			r.isInElection.mu.Unlock()
		case <-time.After(timeoutInterval):
			fmt.Printf("Replica %d: become the coordinator\n", r.id)

			r.shutdownDataChan <- struct{}{}
			r.shutdownTimeoutChan <- struct{}{}

			r.isInElection.mu.Lock()
			r.isInElection.Set(false)
			r.isInElection.mu.Unlock()

			r.isCoordinator.Set(true)

			r.startSync()

		}
	}()

}

func (r *Replica) stopDataListening() {

}

func main() {
	var replicas []*Replica
	allR := make(map[int][]chan string, numReplicas)
	for i := 0; i < numReplicas; i++ {
		currR := &Replica{
			id:   i,
			data: fmt.Sprintf("data_%d", i),

			shutdownSignalChan:     make(chan struct{}, 1),
			shutdownDataChan:       make(chan struct{}, 1),
			shutdownTimeoutChan:    make(chan struct{}, 1),
			shutdownSyncChan:       make(chan struct{}, 1),
			resetTimeoutSignalChan: make(chan struct{}, 1),

			isAlive:       ConcurrentBoolean{value: true},
			isInElection:  ConcurrentBoolean{value: false},
			isCoordinator: ConcurrentBoolean{value: false},
		}
		replicas = append(replicas, currR)
		allR[i] = make([]chan string, 2)
		allR[i][0] = make(chan string) // data
		allR[i][1] = make(chan string) // election
	}
	for i := 0; i < numReplicas; i++ {
		replicas[i].allReplicaChs = allR
	}
	coordinator := replicas[numReplicas-1]
	coordinator.isCoordinator.Set(true)

	for i := 0; i < numReplicas-1; i++ {
		replicas[i].start()
	}
	time.Sleep(200 * time.Millisecond)
	coordinator.start()

	time.Sleep(1500 * time.Millisecond)
	for i := 0; i < numReplicas; i++ {
		if replicas[i].isCoordinator.Get() {
			replicas[i].shutdownSignalChan <- struct{}{}
			fmt.Printf("stopping %d !!!!!\n", i)
			break
		}
	}

	time.Sleep(30 * time.Second)
	for i := 0; i < numReplicas; i++ {
		if replicas[i].isCoordinator.Get() {
			replicas[i].shutdownSignalChan <- struct{}{}
			fmt.Printf("stopping %d !!!!!\n", i)
			break
		}
	}
	time.Sleep(30 * time.Second)
	for i := 0; i < numReplicas; i++ {
		if replicas[i].isCoordinator.Get() {
			replicas[i].shutdownSignalChan <- struct{}{}
			fmt.Printf("stopping %d !!!!!\n", i)
			break
		}
	}

	select {}
}
