#!/bin/bash

CLUSTER_PREFIX=etcd
CLUSTER_SIZE=1
MEM=1024

while [ "$1" != "" ]; do
    case $1 in
        -p | --prefix )
			shift
        	CLUSTER_PREFIX=$1
        ;;
        -s | --size )
			shift
			CLUSTER_SIZE=$1
			if ! [[ "$1" =~ ^[0-9]+$ ]]; then
	            echo "error: cluster size implies integers only..."
				exit -1
			fi
			if [[ "$((${CLUSTER_SIZE}%2))" -eq 0 ]]; then
				echo "error: cluster size odd please..."
				exit -1
			fi
        ;;
        -h | --help )
			echo "$0 -p|--prefix ETCD_CLUSTER_PREFIX -s|--size #ETCD_CLUSTER_SIZE"
        	exit
        ;;
        * )
			$0 -h
        	exit 1
    esac
    shift
done

echo "cluster size is ${CLUSTER_SIZE}"
echo "cluster prefix is ${CLUSTER_PREFIX}"
echo ""

for (( i=1; i<=${CLUSTER_SIZE}; i++ )); do
   ETCD_CLUSTER[$i]=${CLUSTER_PREFIX}-$i
   ETCD_CLUSTER_UUID[$i]=$(/usr/bin/uuidgen)
done
for (( i=1; i<=${CLUSTER_SIZE}; i++ )); do
   ../../bin/corectl run -m ${MEM}\
		-n ${ETCD_CLUSTER[$i]} \
			-u ${ETCD_CLUSTER_UUID[$i]}
   ETCD_CLUSTER_IP[$i]=$(../../bin/corectl q -i ${ETCD_CLUSTER[$i]})
   ETCD_CLUSTER_UUID[$i]=$(../../bin/corectl q -U ${ETCD_CLUSTER[$i]})
   ../../bin/corectl halt ${ETCD_CLUSTER[$i]}
done

for (( i=1; i<=${CLUSTER_SIZE}; i++ )); do
	ETCD_NODE_NAME=${ETCD_CLUSTER[$i]}
	echo ${ETCD_NODE_NAME}
	ETCD_INITIAL_CLUSTER=""
	for (( j=1; j<=${CLUSTER_SIZE}; j++ ));	do
		ETCD_INITIAL_CLUSTER=${ETCD_CLUSTER[$j]}=http://${ETCD_CLUSTER_IP[$j]}:2380,${ETCD_INITIAL_CLUSTER}
	done
	ETCD_INITIAL_CLUSTER=$(echo ${ETCD_INITIAL_CLUSTER} | /usr/bin/sed -e "s#,\$##")
	VM_CCONFIG=$(/usr/bin/mktemp)
	/usr/bin/sed -e 's#__ETCD_NODE_NAME__#'"${ETCD_NODE_NAME}"'#g' \
		-e 's#__ETCD_INITIAL_CLUSTER__#'"${ETCD_INITIAL_CLUSTER}"'#g' \
		-e 's#__COREOS_PRIVATE_IPV4__#'"${ETCD_CLUSTER_IP[$i]}"'#g' \
			etcd-cloud-config.yaml.tmpl > ${VM_CCONFIG}
	 ../../bin/corectl run -m ${MEM} \
	 	-n ${ETCD_CLUSTER[$i]} -u ${ETCD_CLUSTER_UUID[$i]} -L ${VM_CCONFIG}
	rm -rf ${VM_CCONFIG}
done
sleep 2
../../bin/corectl ssh  ${ETCD_CLUSTER["1"]} "etcdctl cluster-health"
