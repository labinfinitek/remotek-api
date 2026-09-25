package lock

import (
	"fmt"
	"sync"
	"testing"
)

func TestLocal_GetLock(t *testing.T) {
	l := NewLocal()
	wg := sync.WaitGroup{}
	wg.Add(3)
	var l1 *sync.Mutex
	var l2 *sync.Mutex
	var l3 *sync.Mutex
	i := 0
	go func() {
		l1 = l.GetLock("key")
		l1.Lock()
		fmt.Println("l1", i)
		i++
		l1.Unlock()
		wg.Done()
	}()
	go func() {
		l2 = l.GetLock("key")
		l2.Lock()
		fmt.Println("l2", i)
		i++
		l2.Unlock()
		wg.Done()
	}()
	go func() {
		l3 = l.GetLock("key")
		l3.Lock()
		fmt.Println("l3", i)
		i++
		l3.Unlock()
		wg.Done()
	}()
	wg.Wait()

	// La stessa chiave deve dare lo stesso mutex. Le stampe di l1..l3 e di i
	// fuori dal lock sono state tolte: erano letture non sincronizzate, che
	// con -race fanno fallire il test.
	if l1 != l2 || l2 != l3 {
		t.Fatalf("GetLock ha restituito mutex diversi per la stessa chiave")
	}
	if i != 3 {
		t.Fatalf("i = %d, atteso 3", i)
	}
}

// TestLocal_Lock: dieci goroutine incrementano i sotto Lock e UnLock della
// stessa chiave; con -race un accesso fuori dal lock fa fallire il test.
func TestLocal_Lock(t *testing.T) {
	l := NewLocal()
	wg := sync.WaitGroup{}
	m := 10
	wg.Add(m)
	i := 0
	for range m {
		go func() {
			l.Lock("key")
			i++
			l.UnLock("key")
			wg.Done()
		}()
	}
	wg.Wait()
	if i != m {
		t.Fatalf("i = %d, atteso %d", i, m)
	}
}

// TestSyncMap: tre LoadOrStore concorrenti sulla stessa chiave, come in
// GetLock, vedono tutti lo stesso valore e ne salva uno solo.
func TestSyncMap(t *testing.T) {
	m := sync.Map{}
	wg := sync.WaitGroup{}
	var valori [3]any
	var caricati [3]bool
	wg.Add(3)
	for n := range 3 {
		go func() {
			valori[n], caricati[n] = m.LoadOrStore("key", n+1)
			wg.Done()
		}()
	}
	wg.Wait()
	salvati := 0
	for n := range 3 {
		if valori[n] != valori[0] {
			t.Errorf("LoadOrStore %d ha visto %v, il primo %v", n, valori[n], valori[0])
		}
		if !caricati[n] {
			salvati++
		}
	}
	if salvati != 1 {
		t.Errorf("valori salvati %d, atteso 1", salvati)
	}
}
