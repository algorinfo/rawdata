import redis


pool = redis.ConnectionPool(host='localhost', port=6379, db=0)
r = redis.Redis(connection_pool=pool)

# r.xgroup_create(name="default", groupname="grouptest", id=u'$', mkstream=False)
rsp = r.xread({"RD.default": "$"}, count=5, block=0)
print(rsp)
