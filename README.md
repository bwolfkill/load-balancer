1) Have your loadbalancer only server a single server. Accept connections and forward to the server. Get that working first.

2) Give the loadbalancer a list of three servers. Have it forward the connection to a different one each time. Maybe pick a random one. Don't do registration of servers yet, just have a static list.

3) Now add registration. A new server can register with the loadbalancer and gets added to the list. All servers still get equal load.

4) Have the servers send their cpu load to the loadbalancer. The loadbalancer should now send more requests to the servers with lower load. Goal is to have an even load on all servers.

5) Last, add a time out. If a server hasn't send it's cpu load in a while, assume it has crashed and remove it from the load balancing.