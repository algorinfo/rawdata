# Raw Data

**WIP**

References and thinking. 

This project pretends to be a lightweight raw data store based on SQLite. Which could be used distributed or as a single instance. 

Raw data is usually the first place where data starts their journey (1). 

Although S3, GCS or Apache Hadoop are used as data lakes (2) those system are planned for blobs biggers than ~50mb (3). As a personal opinion I think this is usefull if the data already exist between a company and maybe this data is distributed in different stores. The problem emerge when incremental small data need to be ingested as is the case for crawled data.

## Concepts

Brain: Main node which coordinates writes and reads between nodes. 
Volume: A node identifed by an address. Each volume could have differents namespaces, but each namespace has only one SQLite file. Volume, node, barrier or bucket are the same. 
Namespace: A different folder or path used to separate differents domains of data. 
Client: Push and/or pull from the store. 


## Main idea behind Raw data

Data is stored in differents SQLite files used as buckets, or barriers(5). Data is distributed using the Jump hash algorithm[(6)](https://arxiv.org/pdf/1406.2294.pdf) between. Jump hash allows a very simple way to shard data and rebalance adding nodes,  see a proof of concept in python [here](https://github.com/nuxion/jump_poc)

The catch of jump hash is it doesn't support random removal of nodes and buckets names. But in the data domain, usually one expect to grow more than shrink (7). 

The brain, or main node doesn't store any information about the data itself, it only maps keys to nodes. Besides that using the jump algorithm, clients could implement the same algorithm and know in advance where the information is, only if they knows which bucket number belongs to which server. The brain is similar to the Directory server in the facebook paper. 

SQLite is battle tested database. It works well under load, and usually is used as a on-disk file format (8)

The problem with small files is mentioned in the facebook's paper but also in the sqlite page (
9): 
    > We initially stored thousands of files in each directoryof an NFS volume which led to an excessive number ofdisk operations to read even a single image.   Becauseof how the NAS appliances manage directory metadata,placing thousands of files in a directory was extremelyinefficient as the directory’s blockmap was too large tobe cached effectively by the  appliance.   Consequentlyit was common to incur more than 10 disk operations toretrieve a single image. 
    

On the other side, sqlite provides us: 
- consistency guaranties writing files
- A easy way to pull data: we can ask to each volume server for the sqlite file directly. 
- A easy way to backup data
- The option to use SQL lang



## Similar projects and inspiration for this work

1. https://github.com/geohot/minikeyvalue
It use a Go Web server as coordinator that manage keys in a leveldb store and redirect each request to a Nginx server used as volume. 
The problem is the same than before, is not suitable for small data in the long term, but I take the idea to have a easy way to restore the data if something fails. 

2. Some time later, I found that the previous work was based on [this paper](https://www.usenix.org/legacy/event/osdi10/tech/full_papers/Beaver.pdf) 

3. https://github.com/chrislusf/seaweedfs 


4. Google: http://infolab.stanford.edu/~backrub/google.html


## References

1. [Building your data lake](https://cloudblogs.microsoft.com/industry-blog/en-gb/technetuk/2020/04/09/building-your-data-lake-on-azure-data-lake-storage-gen2-part-1/)
2. [Data Lake](https://en.wikipedia.org/wiki/Data_lake) 
3. [Data block in HDFS](https://data-flair.training/blogs/data-block/)
4. [Facebook photo storage](https://www.usenix.org/legacy/event/osdi10/tech/full_papers/Beaver.pdf)
5. [The anatomy of a Large-Scale web search engine](http://infolab.stanford.edu/~backrub/google.html)
6. [A Fast, Minimal Memory, Consistent Hash Algorithm](https://arxiv.org/pdf/1406.2294.pdf)
7. [Consistent Hashing: Algorithmic Tradeoffs](https://dgryski.medium.com/consistent-hashing-algorithmic-tradeoffs-ef6b8e2fcae8)
8. [When to use sqlite](https://www.sqlite.org/whentouse.html)
9. [35% faster than the filesystem](https://www.sqlite.org/fasterthanfs.html)
10. [A proof of concept balancing keys in jump hash](https://github.com/nuxion/jump_poc)
