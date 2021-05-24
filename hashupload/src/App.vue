<template>
  <div id="app">
    <HashUpload />
  </div>
</template>

<script>
import HashUpload from './components/HashUpload.vue'

export default {
  name: 'App',
  components: {
    HashUpload
  }
}
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-align: center;
  color: #2c3e50;
  margin-top: 60px;
}
</style>
