<script setup lang="ts">
// Shared shell for all seller auth pages (login / register / forgot / reset).
// Left: full-height cover image. The headline sits at the TOP and the bullet
// points at the BOTTOM, so the middle of the image (the subject) stays clear.
// Right: a prominent brand header followed by the form (default slot).
//
// Swap the cover: drop your own image into src/assets and change the import
// below — everything else keeps working.
import coverUrl from '@/assets/seller-bg.jpg'
import BrandLogo from '@/components/BrandLogo.vue'
import { BRAND } from '@carlitos/shared'

withDefaults(defineProps<{
  title: string              // right-panel form heading
  subtitle?: string          // right-panel form subheading
  panelHeadline?: string     // deprecated: headline no longer rendered over the image
  panelPoints?: string[]     // bullet points shown over the cover image
}>(), {
  subtitle: '',
  panelHeadline: '',
  panelPoints: () => [],
})
</script>

<template>
  <div class="flex min-h-screen bg-white">
    <!-- ── LEFT: cover image + dynamic copy ─────────────────── -->
    <div class="relative hidden w-1/2 overflow-hidden lg:block xl:w-[52%]">
      <img :src="coverUrl" :alt="BRAND" class="absolute inset-0 h-full w-full object-cover" />
      <!-- Top + bottom scrims for text legibility; middle stays clear so the
           image subject is fully visible. -->
      <div class="absolute inset-x-0 top-0 h-2/5 bg-gradient-to-b from-stone-950/80 to-transparent" />
      <div class="absolute inset-x-0 bottom-0 h-2/5 bg-gradient-to-t from-stone-950/85 to-transparent" />

      <div class="relative flex h-full flex-col justify-end p-12">
        <!-- Bullets + copyright at the BOTTOM (no headline) -->
        <div class="max-w-md">
          <ul v-if="panelPoints.length" class="space-y-3.5">
            <li
              v-for="pt in panelPoints"
              :key="pt"
              class="flex items-start gap-2.5 text-[15px] font-medium leading-relaxed text-white"
            >
              <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-[#e63946]" />
              {{ pt }}
            </li>
          </ul>
          <p class="mt-8 text-xs text-stone-300">© {{ new Date().getFullYear() }} {{ BRAND }} · Seller Desk</p>
        </div>
      </div>
    </div>

    <!-- ── RIGHT: brand header + form ───────────────────────── -->
    <div class="flex flex-1 flex-col overflow-y-auto">
      <!-- Prominent brand header above the form -->
      <header class="flex items-center gap-3 border-b border-stone-100 px-5 py-4 sm:px-10 sm:py-5">
        <BrandLogo :height="36" class="sm:hidden" />
        <BrandLogo :height="42" class="hidden sm:block" />
        <div class="border-l border-stone-200 pl-3">
          <p class="text-sm font-semibold leading-tight text-ink">Seller Desk</p>
          <p class="text-xs leading-tight text-stone-400">Manage your store</p>
        </div>
      </header>

      <div class="flex flex-1 items-center justify-center px-5 py-8 sm:px-10 sm:py-10">
        <div class="w-full max-w-md">
          <h1 class="text-xl font-semibold text-ink sm:text-2xl">{{ title }}</h1>
          <p v-if="subtitle" class="mt-1.5 text-sm text-stone-500">{{ subtitle }}</p>

          <div class="mt-6 sm:mt-8">
            <slot />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
