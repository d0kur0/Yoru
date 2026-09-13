import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import test from "node:test";
import ts from "typescript";

function moduleUrl(file, replace = (text) => text) {
  const source = replace(readFileSync(new URL(file, import.meta.url), "utf8"));
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2020,
      module: ts.ModuleKind.ESNext,
    },
  });
  return (
    "data:text/javascript;base64," + Buffer.from(outputText).toString("base64")
  );
}
const modelUrl = moduleUrl("./src/model.ts");
const model = await import(modelUrl);
const { createBackup, parseBackup } = await import(
  moduleUrl("./src/backup.ts", (s) =>
    s.replace('"./model"', JSON.stringify(modelUrl)),
  )
);
const configuration = {
  servers: model.initialServers,
  rules: model.initialRules,
  settings: model.initialSettings,
  selected: "hel",
};
const roundTrip = (value) => parseBackup(JSON.stringify(value));

test("backup round trip preserves all settings, selection and ordered lists", () => {
  const custom = structuredClone(configuration);
  custom.servers.reverse();
  custom.rules.reverse();
  custom.settings.notify = false;
  custom.settings.theme = "dark";
  custom.settings.dnsPolicies[0].domains = ["vpn.internal.example"];
  assert.deepEqual(roundTrip(createBackup(custom)).configuration, custom);
});
test("rejects invalid JSON, unrelated files and future versions", () => {
  assert.throws(() => parseBackup("{"));
  assert.throws(() => roundTrip({ demo: true }));
  assert.throws(() =>
    roundTrip({ ...createBackup(configuration), version: 99 }),
  );
});
test("rejects broken settings, rules, references and duplicate ids", () => {
  for (const mutate of [
    (c) => {
      c.settings.notify = "yes";
    },
    (c) => {
      delete c.settings.dns;
    },
    (c) => {
      c.settings.defaultRoute = "work";
    },
    (c) => {
      c.rules[0].kind = "__proto__";
    },
    (c) => {
      c.rules[0].route = "work";
    },
    (c) => {
      c.servers[0].latency = -1;
    },
    (c) => {
      c.servers.push(c.servers[0]);
    },
    (c) => {
      c.rules.push(c.rules[0]);
    },
    (c) => {
      c.selected = "missing";
    },
  ]) {
    const c = structuredClone(configuration);
    mutate(c);
    assert.throws(() => roundTrip(createBackup(c)));
  }
});
test("accepts an empty server pool and preserves disabled rules", () => {
  const c = { ...configuration, servers: [], selected: "" };
  assert.deepEqual(roundTrip(createBackup(c)).configuration, c);
});
test("unknown fields are not restored", () => {
  const b = createBackup(structuredClone(configuration));
  b.configuration.settings.extra = "ignored";
  assert.equal(roundTrip(b).configuration.settings.extra, undefined);
});
test("restore persists atomically and normal saves keep other fields", () => {
  const values = new Map();
  let fail = false;
  globalThis.localStorage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => {
      if (fail) throw new Error("quota");
      values.set(key, value);
    },
  };
  values.set("tiho.design.v2.selected", JSON.stringify("ams"));
  assert.equal(model.readSaved("selected", ""), "ams");
  model.saveConfiguration(configuration);
  assert.equal(model.readSaved("selected", ""), "hel");
  model.save("selected", "fra");
  assert.deepEqual(model.readSaved("settings", {}), configuration.settings);
  fail = true;
  assert.throws(() =>
    model.saveConfiguration({ ...configuration, selected: "sto" }),
  );
  assert.equal(model.readSaved("selected", ""), "fra");
});

test("v1 backup migrates the previous domain DNS group without losing settings", () => {
  const b = createBackup(structuredClone(configuration));
  b.version = 1;
  delete b.configuration.settings.dnsPolicies;
  b.configuration.settings.dnsDomains = "example.com, *.local";
  b.configuration.settings.domainDns = "10.0.0.2, 10.0.0.3";
  const migrated = roundTrip(b);
  assert.equal(migrated.version, 2);
  assert.deepEqual(migrated.configuration.settings.dnsPolicies[0].domains, [
    "+.example.com",
    "*.local",
  ]);
  assert.deepEqual(migrated.configuration.settings.dnsPolicies[0].servers, [
    "10.0.0.2",
    "10.0.0.3",
  ]);
  assert.equal(migrated.configuration.settings.domainDns, undefined);
});
test("backup preserves independent DNS groups and disabled mappings", () => {
  const c = structuredClone(configuration);
  c.settings.dnsPolicies[0].enabled = false;
  assert.deepEqual(roundTrip(createBackup(c)).configuration, c);
});
test("rejects invalid and duplicate DNS mappings in backups", () => {
  for (const mutate of [
    (c) => {
      c.settings.dnsPolicies[0].servers = ["999.1.1.1"];
    },
    (c) => {
      c.settings.dnsPolicies[0].domains = [];
    },
    (c) => {
      c.settings.dnsPolicies[0].domains = ["https://example.com"];
    },
    (c) => {
      c.settings.dnsPolicies[1].domains = c.settings.dnsPolicies[0].domains;
    },
    (c) => {
      c.settings.dnsPolicies[0].servers = ["udp://10.0.0.1:99999"];
    },
  ]) {
    const c = structuredClone(configuration);
    mutate(c);
    assert.throws(() => roundTrip(createBackup(c)));
  }
});
test("DNS fields accept explicit Mihomo upstream addresses and domain patterns", () => {
  for (const value of [
    "1.1.1.1",
    "10.20.0.53:53",
    "udp://10.20.0.53:53#DIRECT",
    "https://dns.example.com/dns-query",
    "tls://1.1.1.1:853",
    "[::1]:53",
  ])
    assert.equal(model.validateDnsServer(value), true, value);
  for (const value of ["vpn.company.example", "+.company.example", "*.example.com", "localhost"])
    assert.equal(model.validateDnsPattern(value), true, value);
  for (const value of ["https://example.com", "10.0.0.0/8", "bad domain", "+."])
    assert.equal(model.validateDnsPattern(value), false, value);
});

test("domain composer encodes scope and preserves pasted YAML masks", async () => {
  const { domainPattern, domainInfo } = await import(
    moduleUrl("./src/dnsEntryModel.ts")
  );
  assert.equal(domainPattern("Example.COM", "all"), "+.example.com");
  assert.equal(domainPattern("vpn.example.com", "exact"), "vpn.example.com");
  assert.equal(domainPattern("example.com", "level"), "*.example.com");
  assert.equal(domainPattern("*.example.com", "all"), "*.example.com");
  assert.equal(domainPattern("+.example.com", "exact"), "+.example.com");
  assert.deepEqual(domainInfo("*.example.com"), {
    scope: "level",
    name: "example.com",
  });
  assert.deepEqual(domainInfo("+.example.com"), {
    scope: "all",
    name: "example.com",
  });
  assert.deepEqual(domainInfo("vpn.example.com"), {
    scope: "exact",
    name: "vpn.example.com",
  });
});
