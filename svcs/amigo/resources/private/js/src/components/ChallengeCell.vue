<template>
<span class="icon" v-bind:class="{ 'has-text-success': completed }"><i class="fas" v-bind:class="{ 'fa-check-circle': completed, 'fa-circle': !completed }"></i></span>
</template>

<script>
/* eslint-disable */
export default {
  name: 'challenge-cell',
  props: {
    completed: Boolean,
  },
}
</script>
