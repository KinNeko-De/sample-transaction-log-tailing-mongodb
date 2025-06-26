# Transaction Log Tailing with MongoDB

Need reliable event publication without atomic database transactions?

MongoDB's change streams simplify the implementation of the Transaction Log Tailing pattern and empower you to build resilient, event-driven architectures where events are never lost.

Read my [article](https://medium.com/@kinneko-de/99680fefbbac), where I explain the concept in detail.

## How to start

1. Execute the script `./run-sut.sh` to start the MongoDB replica set. 
2. Wait until you see `mongodb-init exited with code 0` in the terminal.
3. Your MongoDB replica set is now ready and you can connect to it.

