package main

import (
  "fmt"
  "time"

  "github.com/QuantumNous/new-api/common"
  "github.com/QuantumNous/new-api/model"
  "github.com/QuantumNous/new-api/service"
  _ "github.com/QuantumNous/new-api/setting/performance_setting"
  "github.com/QuantumNous/new-api/setting/ratio_setting"
)

func main() {
  ratio_setting.InitRatioSettings()
  common.InitEnv()
  service.InitHttpClient()
  service.InitTokenEncoders()

  if err := model.InitDB(); err != nil {
    panic(err)
  }
  model.InitOptionMap()
  if err := model.InitLogDB(); err != nil {
    panic(err)
  }

  if !service.TriggerAggregationRefresh() {
    fmt.Println("refresh_not_triggered")
    return
  }

  fmt.Println("refresh_triggered")
  time.Sleep(8 * time.Second)
}
