# tzndn

[TZDB](https://github.com/eggert/tz) distribution over [NDN](https://named-data.net).  
While this repository focuses on applying NDN to TZDB distribution, this solution can apply to any few producer, massively multiple consumer distributions.

## Design
Using NDN's [State Vector Sync At Least Once (SVS-ALO)](https://github.com/named-data/ndnd/tree/main/std/sync) transport mechanism, a few producers can publish data to many consumers at the same time. 

Both producers and consumers will have to bootstrap on to the [NDN testbed](https://named-data.net/ndn-testbed/). This requires users to generate their testbed certificates using NDNCERT. Users' identity is determined by their email address; NDNCERT can verify identity by comparing a numeric code sent via email. This is an unfortunate requirement; in order for users to register prefixes (receive data), they must have a testbed identity. This is enforced by the operators of the NDN testbed nodes.

A producer will run the `producer` executable. Producers may choose which files (in some TZ directory) to publish. 

A consumer will (as a daemon) run the `consumer` executable. Consumers may choose which files to subscribe to (e.g., only subscribing to `northamerica` if you live in Los Angeles). This will write files to some output directory, in which users may compile the timezone info using the `zic` executable.

Downstream distributors (e.g., Unicode CLDR) may choose to act as both consumers and producers. On a new release, CLDR may choose to republish their own files under some prefix.

## Security
`tzndn` relies on the NDN testbed as the ultimate source of trust. All user certificates are signed by the testbed root certificate (`app/testbed.root.cert`). More work is needed to allow for graceful migration when the testbed certificate expires. 

Enforced by a trust schema (`app/schema.trust`), only publications by certain users will be accepted. As publishers sign each publication with their testbed certificate, consumers will only accept publications signed by certain testbed certificates.

Keys and certificates are cached on device in the `./keychain` directory (relative to where the executable is ran).

## Caching
Producers will not always be online (although they may choose to by keeping the `producer` executable running). This application supports [NDN Repo](https://github.com/named-data/ndnd/tree/main/repo), a transient network store that will serve publications even when the producers are offline.


## Setup
0. Make sure Go 1.24.0 is installed. Python 3.10 is required to make updates to the trust schema.
1. If you're not `eggert@cs.ucla.edu` or `omar@cs.ucla.edu`, you'll have to make updates to the trust schema so that users will verify you as an approved publisher. Run `make schema` to update the schema.
2. Modify `config/main.go`. The most important variables to modify are the `OutputDir` (if a consumer), the `InputDir` (if a producer), and the `UserPrefix`.
3. Run `make` to produce a `producer` executable in `bin/producer` and a `consumer` executable in `bin/consumer`.

## Operation
Both executables take two arguments, the first being the files to publish/subscribe to (not a path, just the filename). `all` will publish/subscribe to all the files listed in `config/main.go`. 

The second argument is your email identity. When running the executable, NDNCERT will prompt you for the verification code sent to your email.

