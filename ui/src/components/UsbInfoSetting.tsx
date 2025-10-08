import { useMemo , useCallback , useEffect, useState } from "react";

import { Button } from "@components/Button";
import { SettingsItem } from "@components/SettingsItem";

import { UsbConfigState } from "../hooks/stores";
import { JsonRpcResponse, useJsonRpc } from "../hooks/useJsonRpc";
import notifications from "../notifications";

import { InputFieldWithLabel } from "./InputField";
import { SelectMenuBasic } from "./SelectMenuBasic";
import Fieldset from "./Fieldset";

function generateJetSerial() {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
  let body = "";
  for (let i = 0; i < 12; i++) {
    body += chars[Math.floor(Math.random() * chars.length)];
  }
  return `JET-${body}`;
}
const generatedSerialNumber = generateJetSerial();

export interface USBConfig {
  vendor_id: string;
  product_id: string;
  serial_number: string;
  manufacturer: string;
  product: string;
}

const usbConfigs = [
  {
    label: "JetKVM Default",
    value: "USB Emulation Device",
  },
  {
    label: "Logitech Universal Adapter",
    value: "Logitech USB Input Device",
  },
  {
    label: "Microsoft Wireless MultiMedia Keyboard",
    value: "Wireless MultiMedia Keyboard",
  },
  {
    label: "Dell Multimedia Pro Keyboard",
    value: "Multimedia Pro Keyboard",
  },
];

type UsbConfigMap = Record<string, USBConfig>;

export function UsbInfoSetting() {
  const { send } = useJsonRpc();
  const [loading, setLoading] = useState(false);

  const [usbConfigProduct, setUsbConfigProduct] = useState("");
  const [deviceId, setDeviceId] = useState("");
  const usbConfigData: UsbConfigMap = useMemo(
    () => ({
      "USB Emulation Device": {
        vendor_id: "0x1d6b",
        product_id: "0x0104",
        serial_number: deviceId || generatedSerialNumber,
        manufacturer: "JetKVM",
        product: "USB Emulation Device",
      },
      "Logitech USB Input Device": {
        vendor_id: "0x046d",
        product_id: "0xc52b",
        serial_number: generatedSerialNumber,
        manufacturer: "Logitech (x64)",
        product: "Logitech USB Input Device",
      },
      "Wireless MultiMedia Keyboard": {
        vendor_id: "0x045e",
        product_id: "0x005f",
        serial_number: generatedSerialNumber,
        manufacturer: "Microsoft",
        product: "Wireless MultiMedia Keyboard",
      },
      "Multimedia Pro Keyboard": {
        vendor_id: "0x413c",
        product_id: "0x2011",
        serial_number: generatedSerialNumber,
        manufacturer: "Dell Inc.",
        product: "Multimedia Pro Keyboard",
      },
    }),
    [deviceId],
  );

  const syncUsbConfigProduct = useCallback(() => {
    send("getUsbConfig", {}, (resp: JsonRpcResponse) => {
      if ("error" in resp) {
        console.error("Failed to load USB Config:", resp.error);
        notifications.error(
          `Failed to load USB Config: ${resp.error.data || "Unknown error"}`,
        );
      } else {
        const usbConfigState = resp.result as UsbConfigState;
        console.log("syncUsbConfigProduct#getUsbConfig result:", usbConfigState);
        const product = usbConfigs.map(u => u.value).includes(usbConfigState.product)
          ? usbConfigState.product
          : "custom";
        setUsbConfigProduct(product);
      }
    });
  }, [send]);

  const handleUsbConfigChange = useCallback(
    (usbConfig: USBConfig) => {
      setLoading(true);
      send("setUsbConfig", { usbConfig }, async (resp: JsonRpcResponse) => {
        if ("error" in resp) {
          notifications.error(
            `Failed to set usb config: ${resp.error.data || "Unknown error"}`,
          );
          setLoading(false);
          return;
        }

        // We need some time to ensure the USB devices are updated
        await new Promise(resolve => setTimeout(resolve, 2000));
        setLoading(false);
        notifications.success(
          `USB Config set to ${usbConfig.manufacturer} ${usbConfig.product}`,
        );

        syncUsbConfigProduct();
      });
    },
    [send, syncUsbConfigProduct],
  );

  useEffect(() => {
    send("getDeviceID", {}, (resp: JsonRpcResponse) => {
      if ("error" in resp) {
        return notifications.error(
          `Failed to get device ID: ${resp.error.data || "Unknown error"}`,
        );
      }
      setDeviceId(resp.result as string);
    });

    syncUsbConfigProduct();
  }, [send, syncUsbConfigProduct]);

  return (
    <Fieldset disabled={loading} className="space-y-4">
      <SettingsItem
        loading={loading}
        title="Identifiers"
        description="USB device identifiers exposed to the target computer"
      >
        <SelectMenuBasic
          size="SM"
          label=""
          className="max-w-[192px]"
          value={usbConfigProduct}
          fullWidth
          onChange={(e: any) => {
            if (e.target.value === "custom") {
              setUsbConfigProduct(e.target.value);
            } else {
              const usbConfig = usbConfigData[e.target.value];
              handleUsbConfigChange(usbConfig);
            }
          }}
          options={[...usbConfigs, { value: "custom", label: "Custom" }]}
        />
      </SettingsItem>
      {usbConfigProduct === "custom" && (
        <div className="ml-2 space-y-4 border-l border-slate-800/10 pl-4 dark:border-slate-300/20 ">
          <USBConfigDialog
            loading={loading}
            onSetUsbConfig={usbConfig => handleUsbConfigChange(usbConfig)}
            onRestoreToDefault={() =>
              handleUsbConfigChange(usbConfigData[usbConfigs[0].value])
            }
          />
        </div>
      )}
    </Fieldset>
  );
}

function USBConfigDialog({
  loading,
  onSetUsbConfig,
  onRestoreToDefault,
}: {
  loading: boolean;
  onSetUsbConfig: (usbConfig: USBConfig) => void;
  onRestoreToDefault: () => void;
}) {
  const [usbConfigState, setUsbConfigState] = useState({
    vendor_id: "",
    product_id: "",
    serial_number: "",
    manufacturer: "",
    product: "",
  } as USBConfig);

  const { send } = useJsonRpc();

  const syncUsbConfig = useCallback(() => {
    send("getUsbConfig", {}, (resp: JsonRpcResponse) => {
      if ("error" in resp) {
        console.error("Failed to load USB Config:", resp.error);
      } else {
        setUsbConfigState(resp.result as UsbConfigState);
      }
    });
  }, [send, setUsbConfigState]);

  // Load stored usb config from the backend
  useEffect(() => {
    syncUsbConfig();
  }, [syncUsbConfig]);

  const handleUsbVendorIdChange = (value: string) => {
    setUsbConfigState({ ...usbConfigState, vendor_id: value });
  };

  const handleUsbProductIdChange = (value: string) => {
    setUsbConfigState({ ...usbConfigState, product_id: value });
  };

  const handleUsbSerialChange = (value: string) => {
    setUsbConfigState({ ...usbConfigState, serial_number: value });
  };

  const handleUsbManufacturer = (value: string) => {
    setUsbConfigState({ ...usbConfigState, manufacturer: value });
  };

  const handleUsbProduct = (value: string) => {
    setUsbConfigState({ ...usbConfigState, product: value });
  };

  return (
    <div className="">
      <div className="grid grid-cols-2 gap-4">
        <InputFieldWithLabel
          required
          label="Vendor ID"
          placeholder="Enter Vendor ID"
          pattern="^0[xX][\da-fA-F]{4}$"
          defaultValue={usbConfigState?.vendor_id}
          onChange={(e: any) => handleUsbVendorIdChange(e.target.value)}
        />
        <InputFieldWithLabel
          required
          label="Product ID"
          placeholder="Enter Product ID"
          pattern="^0[xX][\da-fA-F]{4}$"
          defaultValue={usbConfigState?.product_id}
          onChange={(e: any) => handleUsbProductIdChange(e.target.value)}
        />
        <InputFieldWithLabel
          required
          label="Serial Number"
          placeholder="Enter Serial Number"
          defaultValue={usbConfigState?.serial_number}
          onChange={(e: any) => handleUsbSerialChange(e.target.value)}
        />
        <InputFieldWithLabel
          required
          label="Manufacturer"
          placeholder="Enter Manufacturer"
          defaultValue={usbConfigState?.manufacturer}
          onChange={(e: any) => handleUsbManufacturer(e.target.value)}
        />
        <InputFieldWithLabel
          required
          label="Product Name"
          placeholder="Enter Product Name"
          defaultValue={usbConfigState?.product}
          onChange={(e: any) => handleUsbProduct(e.target.value)}
        />
      </div>
      <div className="mt-6 flex gap-x-2">
        <Button
          loading={loading}
          size="SM"
          theme="primary"
          text="Update USB Identifiers"
          onClick={() => onSetUsbConfig(usbConfigState)}
        />
        <Button
          size="SM"
          theme="light"
          text="Restore to Default"
          onClick={onRestoreToDefault}
        />
      </div>
    </div>
  );
}
