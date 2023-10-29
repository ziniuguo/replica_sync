### Question 1

#### Lamport's logical clock
my_server_client_project

#### Vector clock
s_c_proj_2

In this implementation no causality violation will happen so nothing will be printed.

![img.png](img.png)

## Question 2 cases
### 1
Implemented.

### 2
Line 98. Change this timeoutInterval to control which node first finds that the coordinator has left the network. Refer to the 4th case below for more details.

### 3.a
Line 185-190. It does not matter in my implementation as new election will start after nodes not receiving sync messages for some time.
### 3.b
Line 235. Nothing special will happen, as the higher IDs won't need approvals from lower IDs to be the coordinator.
### 4
It is already implemented in this way. All nodes are timed out almost simultaneously. If a node receives election signal after it has already started election process due to timeout, it will simply reply to the sender instead of starting the election again. If the node starts election due to signal from other nodes, the timer will simply stop.
### 5
Line 235: Non-coordinator leaves.

Line 234 & Line 244: Coordinator leaves.