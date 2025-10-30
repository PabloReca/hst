// @ts-check
import { defineConfig, envField } from 'astro/config';
import node from '@astrojs/node';
import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
  output: 'server',
  adapter: node({
    mode: 'standalone'
  }),
  vite: {
    plugins: [tailwindcss()]
  },
  env: {
    schema: {
      MONGO_URI: envField.string({
        context: 'server',
        access: 'secret',
      }),
      MONGO_DATABASE: envField.string({
        context: 'server',
        access: 'secret',
      })
    }
  }
});