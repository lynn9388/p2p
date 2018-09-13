/*
 * Copyright © 2018 Lynn <lynn9388@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"strconv"

	"github.com/lynn9388/p2p"
)

func main() {
	port := flag.Int("port", 9388, "port for server")
	flag.Parse()

	node := p2p.NewNode("localhost" + strconv.Itoa(*port))
	node.StartServer()
	node.JoinNetwork("localhost:9388")
	node.Wait()
}
