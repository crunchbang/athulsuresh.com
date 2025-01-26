+++
title = "Bug Story: It's not you, it's the environment"
author = ["Athul Suresh"]
date = 2020-05-03
draft = false
+++

It all started with a deployment to the production cluster.

It always does. The worst things happen when you deploy to prod.


## Background {#background}

In our production cluster, we use Aerospike as the primary data store, with data being synced to MySQL for long term storage. For the uninitiated, Aerospike is a high speed, distributed key-value NoSQL database which provides a lot of cool features. Check it [out](https://www.aerospike.com/) if you haven't already. In our cluster, all transactional data gets written to or read from AS, with MySQL being used only as a fallback option. We have a dedicated service that sync data from AS to MySQL and keeps things in check. The speed of access and the ability to scale by adding new nodes helps us keep the pressure off our central MySQL datastore.

I was working on a project that migrated one of our legacy use cases from MySQL to Aerospike. Like all legacy software, this one had a bunch of implicit assumptions about the data store baked into the code, the primary one being persistence. A note about Aersopike - typically data are stored in Aerospike records with a **TTL** (Time To Live). The data gets evicted automatically by the Aerospike engine and this reduces a lot of manual garbage collection from our side. Sadly, this would not be a preferable trait for my use case, as the data was expected to be persisted for weeks or even months, while our typical **TTL** was about a week. Fortunately for me, AS provided a way to persist data indefinitely using `-1` as the **TTL**. Yes! Problem solved. This was the least of my worries as I had to abuse Aersopike in ways that would make its creators cry. That is a story for another post.

I made the required changes, tested out the code, and things seemed to have improved drastically. After a round of code review, I was ready for deployment. The deployment progressed as usual. The use-case was served by a set of APIs, so I was monitoring the cluster for 5xx or any usual errors. The whole thing was done in about 10 mins and all the signals from the cluster were green. No 5xx. No uncaught errors. I patted myself on the back for a smooth deployment (those seem to be a rarity in my life these days).


## A series of unfortunate events {#a-series-of-unfortunate-events}

Remember all those movie characters who celebrated early and later gets killed? A similar, but much less gruesome fate awaited me.

It started with the load balancer throwing 5xx. On further investigation, I found that the backend instance was not responding to certain requests. Digging deeper and grepping through the logs, I saw that request processing for one of the APIs for a completely different use-case was causing it. From the logs, it looked like the request was processed midway and then things abruptly stopped.

Weird indeed.

I did not have a lot of time as this was hitting production traffic (Blue-Green deployments, you say? We've never heard of it). So I quickly reverted the code to the previous stable version and dug deeper.

Delving into the code, I saw that processing stopped abruptly at a point where we were inserting some data into Aersopike with a **TTL** of `-1`. A little bit of context here - our internal wrapper over the AS client library had put some checks in place to prevent people from persisting data forever (**TTL** = `-1`). Whenever someone passed in `-1`, it'd quietly change that into 7 days and pass it along to the AS library. This was abused in several places in our code base where `-1` would be passed in since they expected the lib to put in some default value. This would not do for my case and I'd changed the wrapper to pass `-1` as is to the underlying layers. The offending piece of code was one where `-1` was being passed. So I narrowed down my search and tried calling our client wrapper with `-1` on the instance. I was greeted with a Segmentation Fault from the underlying library. Ah ha! Problem solved!

Well, not exactly. Why did I not get this bug while testing? Our deployments process is a little weird. We have a copy of each dependency stashed away in an S3 bucket which we pull during the deployment. I had used the same version of the lib during testing and the bug did not manifest for me. I dug even deeper (God, when will this stop?!!).

On checking the library version in one of the instances, I found that it had an older version of the lib installed. Suspecting something wrong with the deployment, I pulled up the deployment scripts, expecting to see something amiss. The Aerospike client deployment part was pretty straight forward:

```bash
# script set up

cd /usr/src
wget https://s3.xyz.com/abc/aerospike_client.zip -O aerospike_client.zip
unzip aerospike_client.zip
cd <unzipped dir>

# make and install
```

Things looks right. Nothing out of place. I ran just the client deployment script and checked again. It was still showing the older version of the library!

Super werid.

Somehow it dawned on me to check the man pages for unzip. Usually when you uzip a file and if the resulting directory already exists then unzip will prompt you regarding the next course of action. But when the same script is triggered through an ansible role, it'll silently do nothing and move on with the rest of the flow. Therein lies the problem!

The base AMI we were using already had the unzipped folder baked in with the old version of the library. Whenever the deployment script ran, we downloaded the library code, and tried to unzip it. With no-one to tell it what to do, unzip silently did nothing. Not a single thing. Looking closer, I found that this had been the case since 2016. For 4 years, we had happily deployed code with not a single soul knowing that things were not being deployed as expected.

The fix was a simple addition of the `-o` flag to the unzip command so that it could pummel through anyone and anything that stood in its way.

This seemingly innocuous bug took me from high level application code, to Aerospike client library code, and then, down to our deployment script. All of this because someone did not explicitly instruct unzip to replace while extracting its contents.

All for want of a `-o` flag.


## Lessons Learned {#lessons-learned}

-   Don't deploy on a Friday. Have some heart and think about your on-call engineers.
-   Things can blow up in your face. Be ready to log it when it happens. I had to manually test the client wrapper to find that it was a SEGFAULT.
-   Don't always assume the fault is in your code. Never blindly trust client libraries to do the right thing. We're all human after all.
-   Don't put conflicting defaults in client wrapper code.
-   Don't be an idiot like me and try to change those defaults. Once out in the wild, every perceivable behavior of a lib will be (ab)used by programmers.
-   Read the Frickin' Manual &amp; Be EXPLICIT with your command. Bash has enough red tape around it as is. Make sure that your favorite tools behave the way you
    expect when you plug it into a script. Always err on the side of verbosity and add flags to ensure the expected behavior.
-   Always be ready to dig further. You will most definitely end up learning a lot with a good story to boot.

Ping me your thoughts and comments.
