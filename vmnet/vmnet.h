#ifndef vmnet_h
#define vmnet_h

#include <sys/uio.h>
#include <vmnet/vmnet.h>

void _vmnet_start(interface_ref *interface, uint64_t *max_packet_size);
void _vmnet_stop(interface_ref interface);
void _vmnet_write(interface_ref interface, void *bytes, size_t bytes_size);
void _vmnet_read(interface_ref interface, uint64_t max_packet_size,
                 void **bytes, size_t *bytes_size);

#endif /* vmnet_h */
