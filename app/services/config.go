// Copyright 2017 Vckai Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"strconv"
	"sync"

	"github.com/vckai/novel/app/models"
	"github.com/vckai/novel/app/utils"
)

type Config struct {
	sync.RWMutex

	cfg map[string]string
}

func NewConfig() *Config {
	c := &Config{cfg: make(map[string]string)}
	c.Init()

	return c
}

func (this *Config) Init() {
	// 获取配置
	list := models.ConfigModel.GetAll()

	for _, v := range list {
		this.cfg[v.Key] = v.Value
	}
}

// 获取单个配置
func (this *Config) getData(key string) string {
	this.RLock()
	defer this.RUnlock()

	if c, ok := this.cfg[key]; ok {
		return c
	}

	return ""
}

func (this *Config) String(key string, def ...string) string {
	val := this.getData(key)
	if val != "" {
		return val
	}

	if len(def) > 0 {
		return def[0]
	}

	return ""
}

func (this *Config) Int(key string, def ...int) int {
	val, err := strconv.Atoi(this.getData(key))
	if err == nil {
		return val
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (this *Config) Int64(key string, def ...int64) int64 {
	val, err := strconv.ParseInt(this.getData(key), 10, 64)
	if err == nil {
		return val
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (this *Config) Bool(key string, def ...bool) bool {
	val, err := utils.ParseBool(this.getData(key))
	if err == nil {
		return val
	}

	if len(def) > 0 {
		return def[0]
	}

	return false
}

// 更新配置
func (this *Config) Set(key string, value string) error {
	this.Lock()
	defer this.Unlock()

	if _, ok := this.cfg[key]; !ok {
		return nil
	}

	this.cfg[key] = value

	return models.ConfigModel.Update(key, value)
}

// 获取所有配置
func (this *Config) GetAll() map[string]string {
	return this.cfg
}

// SetOrCreate 写入配置，键不存在时创建
//
// Set 对未知键会静默返回 nil（不报错也不写入），这在新增配置项时
// 容易造成「保存成功但值没变」的困惑。需要动态新增配置项时用本方法。
func (this *Config) SetOrCreate(key, value string) error {
	this.Lock()
	_, exists := this.cfg[key]
	if exists {
		this.cfg[key] = value
	}
	this.Unlock()

	if !exists {
		// 交由模型层插入
		if err := models.ConfigModel.InsertKey(key, value); err != nil {
			return err
		}

		this.Lock()
		this.cfg[key] = value
		this.Unlock()

		return nil
	}

	return models.ConfigModel.Update(key, value)
}
