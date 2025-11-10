package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// taken from a thinkpad l16gen2
var mock_proc_interrupts_file_1 string = `            CPU0       CPU1       CPU2       CPU3       CPU4       CPU5       CPU6       CPU7       CPU8       CPU9       CPU10      CPU11      CPU12      CPU13      
   1:          0          0          0          0          0          0          0          0       4764          0          0          0          0          0  IR-IO-APIC    1-edge      i8042
   8:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-IO-APIC    8-edge      rtc0
   9:     223353          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-IO-APIC    9-fasteoi   acpi
  12:          0          0          0          0          0          0          0      12702          0          0          0          0          0          0  IR-IO-APIC   12-edge      i8042
  14:          0          0          0          0          0          0          0          0          0          0     145052          0          0          0  IR-IO-APIC   14-fasteoi   INTC105E:00
  16:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-IO-APIC   16-fasteoi   processor_thermal_device_pci
  18:       6105          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-IO-APIC   18-fasteoi   i801_smbus
  32:          0    4626565          0          0          0          0          0          0          0          0          0          0          0          0  IR-IO-APIC   32-fasteoi   idma64.0, i2c_designware.0
  33:          0          0          0          0          0      17547          0          0          0          0          0          0          0          0  IR-IO-APIC   33-fasteoi   idma64.1, i2c_designware.1
 113:          0          0          0          0          0          0        249          0          0          0          0          0          0          0  IR-IO-APIC  113-fasteoi   ELAN901C:00
 120:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  DMAR-MSI    0-edge      dmar0
 121:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  DMAR-MSI    1-edge      dmar1
 122:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSI-0000:00:06.0    0-edge      PCIe PME, pciehp
 123:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSI-0000:00:07.0    0-edge      PCIe PME, pciehp
 124:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSI-0000:00:07.2    0-edge      PCIe PME, pciehp
 125:          0          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSI-0000:00:1c.0    0-edge      PCIe PME
 126:          0          0          0          0        102          0          0          0          0          0          0          0          0          0  IR-PCI-MSI-0000:00:16.0    0-edge      mei_me
 127:          0          0          0          0          0          0          0          0          0          0          0          0       2035          0  IR-PCI-MSIX-0000:00:0d.2    0-edge      thunderbolt
 128:          0          0          0          0          0          0          0          0          0          0          0          0          0       2020  IR-PCI-MSIX-0000:00:0d.2    1-edge      thunderbolt
 143:          0          0    3843482          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSI-0000:00:0d.0    0-edge      xhci_hcd
 151:          0          0          0          0          0          0          0          0          0          0     145051          0          0          0  intel-gpio  139  ELAN06DA:00
 152:          0          0          0          0          0          0          0          0          0          0       1557          0          0          0  IR-PCI-MSIX-0000:04:00.0    0-edge      nvme0q0
 153:          0          0          0          0          0          0          0          0          0          0   30528920          0          0          0  IR-PCI-MSI-0000:00:14.0    0-edge      xhci_hcd
 161:      10779          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    1-edge      nvme0q1
 162:          0       6045          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    2-edge      nvme0q2
 163:          0          0      13328          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    3-edge      nvme0q3
 164:          0          0          0       6710          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    4-edge      nvme0q4
 165:          0          0          0          0      24190          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    5-edge      nvme0q5
 166:          0          0          0          0          0      18250          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    6-edge      nvme0q6
 167:          0          0          0          0          0          0      18457          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    7-edge      nvme0q7
 168:          0          0          0          0          0          0          0      18047          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    8-edge      nvme0q8
 169:          0          0          0          0          0          0          0          0      19508          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    9-edge      nvme0q9
 170:          0          0          0          0          0          0          0          0          0      13380          0          0          0          0  IR-PCI-MSIX-0000:04:00.0   10-edge      nvme0q10
 171:          0          0          0          0          0          0          0          0          0          0      15187          0          0          0  IR-PCI-MSIX-0000:04:00.0   11-edge      nvme0q11
 172:          0          0          0          0          0          0          0          0          0          0          0      14357          0          0  IR-PCI-MSIX-0000:04:00.0   12-edge      nvme0q12
 173:          0          0          0          0          0          0          0          0          0          0          0          0       1100          0  IR-PCI-MSIX-0000:04:00.0   13-edge      nvme0q13
 174:          0          0          0          0          0          0          0          0          0          0          0          0          0       1118  IR-PCI-MSIX-0000:04:00.0   14-edge      nvme0q14
 175:          0          0          0          0          0          0          0       2067          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:0d.3    0-edge      thunderbolt
 176:          0          0          0          0          0          0          0          0       2054          0          0          0          0          0  IR-PCI-MSIX-0000:00:0d.3    1-edge      thunderbolt
 191:          0          0          0          0          0          0          0          0          0   24686118          0          0          0          0  IR-PCI-MSI-0000:00:02.0    0-edge      i915
 192:          0          0          0          0          0          0          0          0          0          0          0       5350          0          0  IR-PCI-MSI-0000:00:1f.6    0-edge      enp0s31f6
 193:          0          0          0          0          0          0          0          0          0          0          0          1          0          0  IR-PCI-MSI-0000:00:0b.0    0-edge      intel_vpu
 194:          0          0          0          0          0          0          0          0          0          0          0          0          0    1526433  IR-PCI-MSIX-0000:00:14.3    0-edge      iwlwifi:default_queue
 195:      59357          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    1-edge      iwlwifi:queue_1
 196:          0      70207          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    2-edge      iwlwifi:queue_2
 197:          0          0      88635          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    3-edge      iwlwifi:queue_3
 198:          0          0          0      51143          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    4-edge      iwlwifi:queue_4
 199:          0          0          0          0      35091          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    5-edge      iwlwifi:queue_5
 200:          0          0          0          0          0      34596          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    6-edge      iwlwifi:queue_6
 201:          0          0          0          0          0          0      55838          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    7-edge      iwlwifi:queue_7
 202:          0          0          0          0          0          0          0      75191          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    8-edge      iwlwifi:queue_8
 203:          0          0          0          0          0          0          0          0      41648          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    9-edge      iwlwifi:queue_9
 204:          0          0          0          0          0          0          0          0          0      72385          0          0          0          0  IR-PCI-MSIX-0000:00:14.3   10-edge      iwlwifi:queue_10
 205:          0          0          0          0          0          0          0          0          0          0     119315          0          0          0  IR-PCI-MSIX-0000:00:14.3   11-edge      iwlwifi:queue_11
 206:          0          0          0          0          0          0          0          0          0          0          0      30805          0          0  IR-PCI-MSIX-0000:00:14.3   12-edge      iwlwifi:queue_12
 207:          0          0          0          0          0          0          0          0          0          0          0          0      45147          0  IR-PCI-MSIX-0000:00:14.3   13-edge      iwlwifi:queue_13
 208:          0          0          0          0          0          0          0          0          0          0          0          0          0      43954  IR-PCI-MSIX-0000:00:14.3   14-edge      iwlwifi:queue_14
 209:         39          0          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3   15-edge      iwlwifi:exception
 210:          0      13782          0          0          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSI-0000:00:1f.3    0-edge      AudioDSP
 NMI:        834        587        939        515        424        392        380        360        392        391        398        386         32         36   Non-maskable interrupts
 LOC:   37873969   18668403   42518274   13381155   48162054   46398282   42637505   39766920   39800042   41293179   41448800   36880178    4142707    5904063   Local timer interrupts
 SPU:          0          0          0          0          0          0          0          0          0          0          0          0          0          0   Spurious interrupts
 PMI:        834        587        939        515        424        392        380        360        392        391        398        386         32         36   Performance monitoring interrupts
 IWI:      19994      15072      46941      12826     402059     382798     403930     375522     724263    9619323     527009     520522        332        213   IRQ work interrupts
 RTR:          0          0          0          0          0          0          0          0          0          0          0          0          0          0   APIC ICR read retries
 RES:    1079158     294215     773627     244181    2425372    1527069    1172222     956847    1088478     876727     788422     825313     258726     186385   Rescheduling interrupts
 CAL:    8415491    7635947    6699816    6033729    7590625    7312805    7032284    6860956    6863645    6871540    6921964    6862215    1949644    1867220   Function call interrupts
 TLB:    4402466    5772400    4816911    5653362    6319120    6375626    6342989    6306070    6325868    6274231    6212144    6438880    2039795    2170508   TLB shootdowns
 TRM:      79884      79737      25109      24936      11680      11680      11680      11680      11680      11680      11680      11680      11680      11680   Thermal event interrupts
 THR:          0          0          0          0          0          0          0          0          0          0          0          0          0          0   Threshold APIC interrupts
 DFR:          0          0          0          0          0          0          0          0          0          0          0          0          0          0   Deferred Error APIC interrupts
 MCE:          0          0          0          0          0          0          0          0          0          0          0          0          0          0   Machine check exceptions
 MCP:        411        412        412        412        412        412        412        412        412        412        412        412        412        412   Machine check polls
 ERR:          0
 MIS:          0
 PIN:       3374       4884       4067       3543       4314       3098       2920       2224       2497       2322       3155       4439        100         46   Posted-interrupt notification event
 NPI:          0          0          0          0          0          0          0          0          0          0          0          0          0          0   Nested posted-interrupt event
 PIW:          0          0          0          0          0          0          0          0          0          0          0          0          0          0   Posted-interrupt wakeup event`

// Taken from a x86-64 qemu VM
var mock_proc_interrupts_file_2 string = `           CPU0       CPU1       CPU2       CPU3       
  1:          0          0        216          0   IO-APIC   1-edge      i8042
  8:          0          0          0          0   IO-APIC   8-edge      rtc0
  9:          0          0          0          0   IO-APIC   9-fasteoi   acpi
 12:          0        144          0          0   IO-APIC  12-edge      i8042
 16:          0          0          0          0   IO-APIC  16-fasteoi   i801_smbus
 24:          0          0          0          0  PCI-MSIX-0000:00:02.0   0-edge      PCIe PME, aerdrv
 25:          0          0          0          0  PCI-MSIX-0000:00:02.1   0-edge      PCIe PME, aerdrv
 26:          0          0          0          0  PCI-MSIX-0000:00:02.2   0-edge      PCIe PME, aerdrv
 27:          0          0          0          0  PCI-MSIX-0000:00:02.3   0-edge      PCIe PME, aerdrv
 28:          0          0          0          0  PCI-MSIX-0000:00:02.4   0-edge      PCIe PME, aerdrv
 29:          0          0          0          0  PCI-MSIX-0000:00:02.5   0-edge      PCIe PME, aerdrv
 30:          0          0          0          0  PCI-MSIX-0000:00:02.6   0-edge      PCIe PME, aerdrv
 31:          0          0          0          0  PCI-MSIX-0000:00:02.7   0-edge      PCIe PME, aerdrv
 32:          0          0          0          0  PCI-MSIX-0000:00:03.0   0-edge      PCIe PME, aerdrv
 33:          0          0          0          0  PCI-MSIX-0000:00:03.1   0-edge      PCIe PME, aerdrv
 34:          0          0          0          0  PCI-MSIX-0000:00:03.2   0-edge      PCIe PME, aerdrv
 35:          0          0          0          0  PCI-MSIX-0000:00:03.3   0-edge      PCIe PME, aerdrv
 36:          0          0          0          0  PCI-MSIX-0000:00:03.4   0-edge      PCIe PME, aerdrv
 37:          0          0          0          0  PCI-MSIX-0000:00:03.5   0-edge      PCIe PME, aerdrv
 38:          0          0          0          0  PCI-MSIX-0000:03:00.0   0-edge      virtio2-config
 39:       1914          0          0          0  PCI-MSIX-0000:03:00.0   1-edge      virtio2-virtqueues
 40:          0          0          0          0  PCI-MSIX-0000:01:00.0   0-edge      virtio1-config
 41:          0     296446          0          0  PCI-MSIX-0000:01:00.0   1-edge      virtio1-input.0
 42:          0          0     232633          0  PCI-MSIX-0000:01:00.0   2-edge      virtio1-output.0
 43:          0          0          0          0  PCI-MSIX-0000:04:00.0   0-edge      virtio3-config
 44:     340662          0          0          0  PCI-MSIX-0000:04:00.0   1-edge      virtio3-req.0
 45:          0     563738          0          0  PCI-MSIX-0000:04:00.0   2-edge      virtio3-req.1
 46:          0          0     663679          0  PCI-MSIX-0000:04:00.0   3-edge      virtio3-req.2
 47:          0          0          0     677646  PCI-MSIX-0000:04:00.0   4-edge      virtio3-req.3
 48:          0         85          0          0  PCI-MSIX-0000:02:00.0   0-edge      xhci_hcd
 53:          0          0          1          0  PCI-MSIX-0000:00:01.0   0-edge      virtio0-config
 54:          0          0          0       4161  PCI-MSIX-0000:00:01.0   1-edge      virtio0-control
 55:       1260          0          0          0  PCI-MSIX-0000:00:01.0   2-edge      virtio0-cursor
 56:          0      60793          0          0  PCI-MSI-0000:00:1f.2   0-edge      ahci[0000:00:1f.2]
 57:          0          0          0          0  PCI-MSIX-0000:06:00.0   0-edge      virtio5-config
 58:          0          0          0       1936  PCI-MSIX-0000:06:00.0   1-edge      virtio5-input
 59:          0          0          0          0  PCI-MSIX-0000:05:00.0   0-edge      virtio4-config
 60:          0          0          0          0  PCI-MSIX-0000:05:00.0   1-edge      virtio4-virtqueues
 61:          0          0        201          0  PCI-MSI-0000:00:1b.0   0-edge      snd_hda_intel:card0
NMI:          0          0          0          0   Non-maskable interrupts
LOC:   11867954   13426126   13738914   13064398   Local timer interrupts
SPU:          0          0          0          0   Spurious interrupts
PMI:          0          0          0          0   Performance monitoring interrupts
IWI:        145         59         24        116   IRQ work interrupts
RTR:          0          0          0          0   APIC ICR read retries
RES:     244200     271609     273101     254866   Rescheduling interrupts
CAL:    3093475    2873071    2855021    2675922   Function call interrupts
TLB:     121788     219623     250939     235661   TLB shootdowns
TRM:          0          0          0          0   Thermal event interrupts
THR:          0          0          0          0   Threshold APIC interrupts
DFR:          0          0          0          0   Deferred Error APIC interrupts
MCE:          0          0          0          0   Machine check exceptions
MCP:        382        384        383        381   Machine check polls
HYP:          1          1          1          1   Hypervisor callback interrupts
ERR:          0
MIS:          0
PIN:          0          0          0          0   Posted-interrupt notification event
NPI:          0          0          0          0   Nested posted-interrupt event
PIW:          0          0          0          0   Posted-interrupt wakeup event`

func TestReadAndParseProcInterrupts(t *testing.T) {

	pid, err := ParseProcInterrupts(mock_proc_interrupts_file_1)
	assert.NoError(t, err)

	// 164:          0          0          0       6710          0          0          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:04:00.0    4-edge      nvme0q4
	line, err := pid.FindInterruptById(164)
	assert.NoError(t, err)
	assert.Equal(t, uint64(6710), line.interrupt_counts_per_cpu[3])
	assert.Equal(t, uint64(0), line.interrupt_counts_per_cpu[0])
	assert.Equal(t, "IR-PCI-MSIX-0000:04:00.0", line.interrupt_controller)
	assert.Equal(t, "04:00.0", line.interrupt_pci_bdf)
	assert.Equal(t, "4-edge", line.interrupt_pin_name_vector)
	assert.Equal(t, "nvme0q4", line.interrupt_device_and_driver_name)

	// 200:          0          0          0          0          0      34596          0          0          0          0          0          0          0          0  IR-PCI-MSIX-0000:00:14.3    6-edge      iwlwifi:queue_6
	line, err = pid.FindInterruptById(200)
	assert.NoError(t, err)
	assert.Equal(t, "00:14.3", line.interrupt_pci_bdf)
	assert.Equal(t, "6-edge", line.interrupt_pin_name_vector)
	assert.Equal(t, "iwlwifi:queue_6", line.interrupt_device_and_driver_name)

	// IWI:      19994      15072      46941      12826     402059     382798     403930     375522     724263    9619323     527009     520522        332        213   IRQ work interrupts
	line, err = pid.FindInterruptByName("IWI")
	assert.NoError(t, err)
	assert.Equal(t, uint64(213), line.interrupt_counts_per_cpu[13])
	assert.Equal(t, "IRQ work interrupts", line.interrupt_acronym_extended)

	// ERR:          0
	line, err = pid.FindInterruptByName("ERR")
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), line.interrupt_count_single)

	_, err = ParseProcInterrupts(mock_proc_interrupts_file_2)
	assert.NoError(t, err)
}
