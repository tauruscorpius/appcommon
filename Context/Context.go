package Context

import (
	"context"
	"time"
)

func _cancel() {}

func GetRedisSessCtx() (context.Context, func()){
	return context.Background(), _cancel
}

func GetTransmitterCtx() (context.Context, func()){
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	return ctx, cancel
}