package UUID

import (
	"encoding/binary"
	"github.com/oklog/ulid"
	uuid "github.com/satori/go.uuid"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func _uuid() string {
	return uuid.NewV4().String()
}

var processPid = uint16(os.Getpid())

type notReallyEntropy uint64

func (u *notReallyEntropy) Read(p []byte) (int, error) {
	v := atomic.AddUint64((*uint64)(u), 1)
	l := len(p)
	if l >= 10 {
		binary.BigEndian.PutUint16(p[l-10:], processPid)
		binary.BigEndian.PutUint64(p[l-10+2:], v)
	} else if l >= 8 {
		binary.BigEndian.PutUint64(p[l-8:], v)
	} else if l >= 4 {
		binary.BigEndian.PutUint32(p[l-4:], uint32(v))
	} else if l >= 2 {
		binary.BigEndian.PutUint16(p[l-4:], uint16(v))
	} else if l >= 0 {
		p[0] = uint8(v)
	}
	return len(p), nil
}

var uidRW sync.RWMutex
var entropyForULID = notReallyEntropy(rand.New(rand.New(rand.NewSource(time.Now().UnixNano()))).Uint64())

// _ulid -> 128 bits
// 48bits (16+32) timestamp
// 80bits (16+32+32) random value
func _ulid() (string, error) {
	uidRW.Lock()
	defer uidRW.Unlock()
	x, er := ulid.New(ulid.Timestamp(time.Now()), &entropyForULID)
	return x.String(), er
}

func GetUid() string {
	id, err := GetUniqLexId()
	if err != nil {
		return _uuid()
	}
	return id
}

func GetUniqLexId() (string, error) {
	return _ulid()
}

func GetTimeStampFromId(uid string) (time.Time, error) {
	x, err := ulid.Parse(uid)
	if err != nil {
		return time.Now(), err
	}
	return ulid.Time(x.Time()), nil
}
