#ifndef KAFKA_MAPS_H
#define KAFKA_MAPS_H

#include "map-defs.h"

#include "protocols/kafka/defs.h"
#include "protocols/kafka/types.h"

// Kernels before 4.7 do not know about per-cpu array maps.
#if LINUX_VERSION_CODE >= KERNEL_VERSION(4, 7, 0)
// A per-cpu buffer used to read requests fragments during protocol
// classification and avoid allocating a buffer on the stack. Some protocols
// requires us to read at offset that are not aligned. Such reads are forbidden
// if done on the stack and will make the verifier complain about it, but they
// are allowed on map elements, hence the need for this map.
BPF_PERCPU_ARRAY_MAP(kafka_client_id, __u32, char [CLIENT_ID_SIZE_TO_VALIDATE], 1)
BPF_PERCPU_ARRAY_MAP(kafka_topic_name, __u32, char [TOPIC_NAME_MAX_STRING_SIZE_TO_VALIDATE], 1)
#else
// Kernels < 4.7.0 do not know about the per-cpu array map used
// in classification, preventing the program to load even though
// we won't use it. We change the type to a simple array map to
// circumvent that.
BPF_ARRAY_MAP(kafka_client_id, __u32, 1)
BPF_ARRAY_MAP(kafka_topic_name, __u32, 1)
#endif

// For parsing

BPF_PERCPU_ARRAY_MAP(kafka_heap, __u32, kafka_transaction_t, 1)
/*
    This map help us to avoid processing the same traffic twice.
    It holds the last tcp sequence number for each connection.
   */
BPF_HASH_MAP(kafka_last_tcp_seq_per_connection, conn_tuple_t, __u32, 0)

#endif
