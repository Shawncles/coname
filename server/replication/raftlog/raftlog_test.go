package raftlog

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/andres-erbsen/clock"
	"github.com/coreos/etcd/raft"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/yahoo/coname/server/kv"
	"github.com/yahoo/coname/server/kv/leveldbkv"
	"github.com/yahoo/coname/server/replication"
	"github.com/yahoo/coname/server/replication/raftlog/proto"
)

const tick = time.Second

func chain(fs ...func()) func() {
	ret := func() {}
	for _, f := range fs {
		// the functions are copied to the heap, the closure refers to a unique copy of its own
		oldRet := ret
		thisF := f
		ret = func() { oldRet(); thisF() }
	}
	return ret
}

func setupDB(t *testing.T) (db kv.DB, teardown func()) {
	dir, err := ioutil.TempDir("", "raftlog")
	if err != nil {
		t.Fatal(err)
	}
	teardown = func() { os.RemoveAll(dir) }
	ldb, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		teardown()
		t.Fatal(err)
	}
	teardown = chain(func() { ldb.Close() }, teardown)
	return leveldbkv.Wrap(ldb), teardown
}

// raft replicas are numbered 1..n  and reside in array indices 0..n-1
func setupRaftLogCluster(t *testing.T, n int) (ret []replication.LogReplicator, clks []*clock.Mock, teardown func()) {
	peers := make([]raft.Peer, 0, n)
	for i := uint64(0); i < uint64(n); i++ {
		peers = append(peers, raft.Peer{ID: 1 + i})
	}

	addrs := make([]string, 0, n)
	lookupDialer := func(id uint64) proto.RaftClient {
		cc, err := grpc.Dial(addrs[id-1])
		if err != nil {
			panic(err) // async dial should not err
		}
		return proto.NewRaftClient(cc)
	}
	teardown = func() {}

	for i := 0; i < n; i++ {
		c := &raft.Config{
			ID:              uint64(1 + i),
			ElectionTick:    10,
			HeartbeatTick:   1,
			MaxSizePerMsg:   4096,
			MaxInflightMsgs: 256,
		}
		db, dbDown := setupDB(t)
		teardown = chain(dbDown, teardown)
		clk := clock.NewMock()
		clks = append(clks, clk)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		l, err := Open(db, nil, c, peers, clk, tick, ln, grpc.NewServer(), lookupDialer)
		if err != nil {
			teardown()
			t.Fatal(err)
		}
		ret = append(ret, l)
		teardown = chain(func() { l.Stop() }, teardown)
	}

	for _, l := range ret {
		addrs = append(addrs, l.(*raftLog).grpcListen.Addr().String())
	}

	for _, l := range ret {
		l.Start(0)
	}
	return ret, clks, teardown
}

func TestRaftLogStartStop1(t *testing.T) {
	_, _, teardown := setupRaftLogCluster(t, 1)
	defer teardown()
}

func TestRaftLogStartStop3(t *testing.T) {
	_, _, teardown := setupRaftLogCluster(t, 3)
	defer teardown()
}

func TestRaftLogStartStop5(t *testing.T) {
	_, _, teardown := setupRaftLogCluster(t, 5)
	defer teardown()
}

type appendMachine struct {
	db  kv.DB
	log replication.LogReplicator

	state        []byte
	nextIndexLog uint64
	get          chan chan []byte

	stopOnce sync.Once
	stop     chan struct{}
	waitStop sync.WaitGroup
}

func openAppendMachine(db kv.DB, log replication.LogReplicator) *appendMachine {
	am := &appendMachine{db: db, log: log, stop: make(chan struct{}), get: make(chan chan []byte)}
	am.load()
	return am
}

func (am *appendMachine) Start() {
	am.waitStop.Add(1)
	go func() { am.run(); am.waitStop.Done() }()
}

func (am *appendMachine) Stop() {
	am.stopOnce.Do(func() {
		close(am.stop)
		am.waitStop.Wait()
	})
}

func (am *appendMachine) Get() []byte {
	ch := make(chan []byte)
	am.get <- ch
	return <-ch
}

func (am *appendMachine) run() {
	for {
		select {
		case ch := <-am.get:
			ch <- append([]byte{}, am.state...)
		case <-am.stop:
			return
		case stepLogEntry := <-am.log.WaitCommitted():
			am.state = append(am.state, stepLogEntry.Data...)
			am.nextIndexLog++
			am.persist()
		}
	}
}

func (am *appendMachine) persist() {
	var idx [8]byte
	binary.BigEndian.PutUint64(idx[:], am.nextIndexLog)
	if err := am.db.Put([]byte{}, append(am.state, idx[:]...)); err != nil {
		panic(err)
	}
}

func (am *appendMachine) load() {
	var err error
	am.state, err = am.db.Get([]byte{})
	if err == am.db.ErrNotFound() {
		return
	}
	if err != nil {
		panic(err)
	}
	am.nextIndexLog = binary.BigEndian.Uint64(am.state[len(am.state)-8:])
	am.state = am.state[:len(am.state)-8]
}

func setupAppendMachineCluster(t *testing.T, n int) (ret []*appendMachine, clks []*clock.Mock, teardown func()) {
	replicas, clks, teardown := setupRaftLogCluster(t, n)
	for _, r := range replicas {
		db, td := setupDB(t)
		am := openAppendMachine(db, r)
		am.Start()
		ret = append(ret, am)
		teardown = chain(am.Stop, td, teardown)
	}
	return ret, clks, teardown
}

func TestAppendMachineStartStop1(t *testing.T) {
	_, _, teardown := setupAppendMachineCluster(t, 1)
	defer teardown()
}

func TestAppendMachineStartStop3(t *testing.T) {
	_, _, teardown := setupAppendMachineCluster(t, 3)
	defer teardown()
}

func TestAppendMachineStartStop5(t *testing.T) {
	_, _, teardown := setupAppendMachineCluster(t, 5)
	defer teardown()
}

func TestAppendMachineEachProposeOneAndStop5(t *testing.T) {
	replicas, _, teardown := setupAppendMachineCluster(t, 5)
	defer teardown()
	for i, am := range replicas {
		go am.log.Propose(context.TODO(), []byte{byte(i)})
	}
}

func isConsistentPrefix(a, b []byte) bool {
	l := len(a)
	if len(b) < l {
		l = len(b)
	}
	return bytes.Equal(a[:l], b[:l])
}

func checkReplicasConsistent(t *testing.T, states map[int][]byte) {
	for i, si := range states {
		for j, sj := range states {
			if !isConsistentPrefix(si, sj) {
				t.Errorf("logs of replicas %d and %d diverged: %s <> %s", 1+i, 1+j, si, sj)
			}
		}
	}
}

func testAppendMachineEachProposeAndWait(t *testing.T, l, n int) {
	replicas, clks, teardown := setupAppendMachineCluster(t, n)
	defer teardown()
	for j := 0; j < l; j++ {
		for i, am := range replicas {
			s := fmt.Sprintf("(%d:%d)", i+1, j)
			go am.log.Propose(context.TODO(), []byte(s))
		}
	}
	states := make(map[int][]byte)
	for len(states) < len(replicas) {
		i := rand.Intn(len(replicas))
		clks[i].Add(tick)
		if s := replicas[i].Get(); len(s) >= len(replicas) {
			if prev, _ := states[i]; !bytes.Equal(s[:len(prev)], prev) {
				t.Fatalf("replica %d changed its history", 1+i)
			}
			states[i] = s
			checkReplicasConsistent(t, states)
		}
	}
	if testing.Verbose() {
		t.Log(string(states[1]))
	}
}

func TestAppendMachineEachPropose1AndWait5(t *testing.T) {
	testAppendMachineEachProposeAndWait(t, 1, 5)
}

func TestAppendMachineEachPropose13AndWait3(t *testing.T) {
	testAppendMachineEachProposeAndWait(t, 13, 3)
}
