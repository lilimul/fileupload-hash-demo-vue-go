<template>
   <div>
    <input type="file" @change="handleFileChange" />
    <el-button @click="handleUpload">上传</el-button>
    <el-button @click="handlePause">{{ uploadInfo }}</el-button>
     <div>
      <div>计算文件 hash</div>
      <el-progress :percentage="hashPercentage"></el-progress>
      <div>总进度</div>
      <el-progress :percentage="fakeUploadPercentage"></el-progress>
    </div>
    <el-table :data="data">
      <el-table-column
        prop="hash"
        label="切片hash"
        align="center"
      ></el-table-column>
      <el-table-column label="大小(KB)" align="center" width="120">
        <template v-slot="{ row }">
          {{ row.size | transformByte }}
        </template>
      </el-table-column>
      <el-table-column label="进度" align="center">
        <template v-slot="{ row }">
          <el-progress
            :percentage="row.percentage"
            color="#909399"
          ></el-progress>
        </template>
      </el-table-column>
    </el-table>
  </div>
  
</template>

<script>
const CHUNK_SIZE = 1024*10;
export default {
  filters: {
    transformByte(val) {
      return Number((val / 1024).toFixed(0));
    }
  },
  data: () => ({
    container: {
      file: null
    },
    url: "http://localhost:8081",
    data:[],
    requestList:[],
    apple:[1,2,3],
    hashPercentage:0,
    fakeUploadPercentage: 0,
    uploadInfo:"等待上传",
    isPausing:false
  }),
  watch:{
     uploadPercentage(now) {
     if (now > this.fakeUploadPercentage) {
        this.fakeUploadPercentage = now;
      }
    }
  },
  computed:{
    uploadPercentage(){
      if(!this.container.file||!this.data.length){
        return 0;
      }
      const loaded = this.data.map(item=>item.size*item.percentage).reduce((acc,cur)=>acc+cur);
      return parseInt((loaded/this.container.file.size).toFixed(2))
    }
  },
  methods: {
     handleFileChange(e) {//update file
      const [file] = e.target.files;
      if (!file) return;
      Object.assign(this.$data, this.$options.data());
      this.container.file = file;
    },
    async handleUpload() {//push upload btn
      if(!this.container.file) return;
      const fileChunkList = this.createFileChunk(this.container.file);
      this.container.hash = await this.caculateHash(fileChunkList)
      console.log("hash:",this.container.hash)
      //verfy if file have uploaded before
       const { ShouldUpload, UploadedChunkIdxs } = await this.verifyUpload(
        this.container.file.name,
        this.container.hash
      );
       if (!ShouldUpload) {
        this.$message.success("秒传：上传成功,在这里添加额外的文件重复映射步骤");
        this.fakeUploadPercentage=100
        return;
      }
      this.data = fileChunkList.map(({file},index)=>({
        chunk:file,
        index,
        hash:this.container.hash+"-"+index,
        fileHash:this.container.hash,
        size:file.size,
        percentage: UploadedChunkIdxs.includes(index) ? 100 : 0
      }));
      await this.uploadChunks();
    },
      async verifyUpload(filename, fileHash) {//check if file chunks or whole file uploaded before upload
       const { data } = await this.request({
         url: "http://localhost:8081/verify",
         headers: {
           "content-type": "application/json"
        },
         data: JSON.stringify({
           "FileName":filename,
           "Hash":fileHash
         })
       });
       return JSON.parse(data)
     },
    createFileChunk(file,size=CHUNK_SIZE){//splice file into small chunks
      const fileChunks=[];
      let cur=0;
      while(cur<file.size){
        fileChunks.push({file:file.slice(cur,cur+size)});
        cur+=size;
      }
      return fileChunks;
    },
    caculateHash(fileChunkList){//caculate hash by worker
      return new Promise(resolve=>{
        this.container.worker =new Worker("/hash.js");
        this.container.worker.postMessage({
          fileChunkList 
        });
        this.container.worker.onmessage = e=>{
          const {percentage,hash} =e.data;
          this.hashPercentage = percentage;
          if (hash){
            resolve(hash);
          }
        }
      })
    },
    handlePause() {//stop all unfinished xhr
    if(!this.isPausing){
    this.requestList.forEach(xhr => xhr?.abort());
    this.requestList=[];
    this.isPausing=true;
    this.uploadInfo="暂停中"
    }else{
      this.uploadInfo="上传中";
      this.isPausing=false;
    }

    },
    async uploadChunks(){//upload chunks
      this.uploadInfo="暂停"
      const requestList = this.data
     .filter(({ percentage }) => {return percentage!=100})//if this chunks have uploadded before, ignore it
      .map(({chunk,hash,index})=>{//upload via form,with name and hash
        const formData = new FormData();
        formData.append("chunk",chunk);
        formData.append("hash",hash);
        formData.append("fileName",this.container.file.name);
        return {formData,index};
      }).map(async ({formData,index})=>{
        this.request({
          url:this.url+"/upload",
          data:formData,
          onProgress:this.createProgressHandler(this.data[index]),//use func gened by factory function as onProgress handler
          requestList:this.requestList
        })
      });
      await Promise.all(requestList);//upload
      await this.mergeRequest();//merge file when all chunks uplaoded
    },
    createProgressHandler(item){
      return e=>{
        item.percentage = parseInt(String((e.loaded/e.total)*100));
      }
    },
    async mergeRequest(){//ask server to merge chunks into a complete file
      await this.request({
        url:this.url+"/merge",
        headers:{
          "content-type": "application/json"
        },
        data:JSON.stringify({
          FileName:this.container.file.name,
          Hash:this.container.hash
        })
      })
    },
    request({
      url,
      method="post",
      data,
      headers={},
      onProgress=e=>e,
      requestList=[]//save for pause or resume
    }){
      return new Promise(resolve=>{
        const xhr= new XMLHttpRequest();
        xhr.open(method,url);
        Object.keys(headers).forEach(key=>{
          xhr.setRequestHeader(key,headers[key]);
        })
        xhr.upload.onprogress=onProgress;
        xhr.send(data);
        xhr.onload=e=>{
          if(requestList.length>0){
            const xhrIndex = requestList.findIndex(item=>item===xhr);
            requestList.splice(xhrIndex,1);
          }
          resolve({
            data:e.target.response
          })
        }
        requestList.push(xhr);
      })
    }
  }
};
</script>