#import "vmnet.h"

void _vmnet_start(interface_ref *interface, uint64_t *max_packet_size) {
  xpc_object_t interface_desc = xpc_dictionary_create(NULL, NULL, 0);
  xpc_dictionary_set_uint64(interface_desc, vmnet_operation_mode_key,
                            VMNET_SHARED_MODE);

  dispatch_queue_t interface_start_queue =
      dispatch_queue_create("vmkit.vmnet.start", DISPATCH_QUEUE_SERIAL);
  dispatch_semaphore_t interface_start_semaphore = dispatch_semaphore_create(0);

  __block interface_ref _interface;
  __block vmnet_return_t interface_status;
  __block uint64_t _max_packet_size = 0;

  _interface = vmnet_start_interface(
      interface_desc, interface_start_queue,
      ^(vmnet_return_t status, xpc_object_t interface_param) {
        interface_status = status;

        if (status == VMNET_SUCCESS) {
          _max_packet_size = xpc_dictionary_get_uint64(
              interface_param, vmnet_max_packet_size_key);
        }

        dispatch_semaphore_signal(interface_start_semaphore);
      });

  dispatch_semaphore_wait(interface_start_semaphore, DISPATCH_TIME_FOREVER);

  dispatch_release(interface_start_queue);
  xpc_release(interface_desc);

  if (interface_status != VMNET_SUCCESS) {
    return;
  }

  *interface = _interface;
  *max_packet_size = _max_packet_size;
}

void _vmnet_stop(interface_ref interface) {
  if (interface == NULL) {
    return;
  }

  dispatch_queue_t interface_stop_queue =
      dispatch_queue_create("vmkit.vmnet.stop", DISPATCH_QUEUE_SERIAL);
  dispatch_semaphore_t interface_stop_semaphore = dispatch_semaphore_create(0);

  vmnet_stop_interface(interface, interface_stop_queue,
                       ^(vmnet_return_t status) {
                         dispatch_semaphore_signal(interface_stop_semaphore);
                       });

  dispatch_semaphore_wait(interface_stop_semaphore, DISPATCH_TIME_FOREVER);
  dispatch_release(interface_stop_queue);
}

void _vmnet_write(interface_ref interface, void *bytes, size_t bytes_size) {
  if (interface == NULL) {
    return;
  }

  struct iovec packets_iovec = {
      .iov_base = bytes,
      .iov_len = bytes_size,
  };

  struct vmpktdesc packets = {
      .vm_pkt_size = bytes_size,
      .vm_pkt_iov = &packets_iovec,
      .vm_pkt_iovcnt = 1,
      .vm_flags = 0,
  };

  int packets_count = packets.vm_pkt_iovcnt;
  vmnet_write(interface, &packets, &packets_count);
}

void _vmnet_read(interface_ref interface, uint64_t max_packet_size,
                 void **bytes, size_t *bytes_size) {
  struct iovec packets_iovec = {
      .iov_base = malloc(max_packet_size),
      .iov_len = max_packet_size,
  };

  struct vmpktdesc packets = {
      .vm_pkt_size = max_packet_size,
      .vm_pkt_iov = &packets_iovec,
      .vm_pkt_iovcnt = 1,
      .vm_flags = 0,
  };

  int packets_count = 1;
  vmnet_return_t status = vmnet_read(interface, &packets, &packets_count);

  if (status != VMNET_SUCCESS || packets_count == 0) {
    return;
  }

  *bytes = packets.vm_pkt_iov->iov_base;
  *bytes_size = packets.vm_pkt_size;
}
