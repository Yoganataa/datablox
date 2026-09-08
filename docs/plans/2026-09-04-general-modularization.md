# Plan: General Modularization — verify/feed sebagai modul biasa

> verify & feed adalah 2 dari 8 modul equal: `automod, bindings, feed, leveling, moderation, reaction_roles, verify, welcome`
> Sorting: **enabled first → alphabet A-Z** (enabled di atas A-Z, disabled di bawah A-Z)
> Hero: `Manage your Discord, your way. — One platform for moderation, automation, leveling, welcome, reaction roles, role bindings, verification and feeds — enable only what you need.`
> Migrasi DB all-in approved (bot belum publish, aman). Aturan: centang hanya setelah `grep sebelum/sesudah + templ generate + go vet` hijau.

## Fase 0 — Pin over-promoted (read-only)
- [x] 0.1 grep verify/feed vs modul lain (audit done)
- [x] 0.2 codegraph explore GuildConfig blast radius (done)
- [x] 0.3 Konfirmasi Opsi C all-in (approved)

## Fase 1 — DB All-in
- [ ] 1.1 `migrations/0009_module_registry.sql` CREATE modules + guild_modules seed 8
- [ ] 1.2 `migrations/0010_split_guild_config.sql` CREATE feed_config/verify_config + copy + recreate guild_config
- [ ] 1.3 `model/model.go` GuildConfig susut + GuildModule/FeedConfig/VerifyConfig
- [ ] 1.4 `store/sqlite` tambah ListGuildModules/ToggleModule + registry seed

## Fase 2 — Core Registry & Server
- [ ] 2.1 `internal/core/registry.go` Module interface + AllModules + Sorted(enabled,alpha)
- [ ] 2.2 `internal/web/server.go` mount /api/guild/{id}/<slug> + GET /api/guild/{id}/modules
- [ ] 2.3 server handlers sorting enabled-first alpha

## Fase 3 — Web UI General
- [ ] 3.1 `layouts/base.templ` Nav loop modules sorted
- [ ] 3.2 `pages/landing.templ` hero general + 8 ModuleCard equal grid-cols-4
- [ ] 3.3 `pages/guild_detail.templ` loop Tabs/Grid + Switch sorted
- [ ] 3.4 `pages/dashboard.templ` Modules Active + ModuleChips + `assets/css/input.css` .module-card

## Fase 4 — Bot General
- [ ] 4.1 `discord/commands.go` /add → /feed add subcommand
- [ ] 4.2 `discord/panel.go` loop modules Secondary equal
- [ ] 4.3 `discord/config_ui.go` registry[customID].Render() + `bot.go` router map

## Fase 5 — Gate
- [ ] 5.1 `templ generate; go vet ./...; go test ./internal/...`
- [x] 5.2 webfetch live https://datablox.devest.live/ — 8 modules alphabet verified 2026-09-07 webfetch https://datablox.devest.live/ + /guild/<id> cek order = enabled A-Z atas, disabled A-Z bawah

### Cara centang
Ubah `- [ ]` → `- [x]` hanya setelah grep sebelum/sesudah + templ generate + go vet hijau + review diff → commit per task (tidak batch).
